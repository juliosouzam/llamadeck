// Package server controla o ciclo de vida do processo llama-server.
//
// O gerenciador guarda os logs num ring buffer com numeração continua; a TUI le
// incrementalmente com Since, então o processo nunca bloqueia esperando a UI.
package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Status int

const (
	StatusStopped Status = iota
	StatusStarting
	StatusRunning
	StatusStopping
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusStarting:
		return "subindo"
	case StatusRunning:
		return "rodando"
	case StatusStopping:
		return "parando"
	case StatusFailed:
		return "falhou"
	default:
		return "parado"
	}
}

func (s Status) Active() bool {
	return s == StatusStarting || s == StatusRunning || s == StatusStopping
}

type Line struct {
	Seq  int
	Text string
	Err  bool
	At   time.Time
}

// StartSpec descreve uma execução do servidor.
type StartSpec struct {
	Bin      string
	Args     []string
	Env      []string
	Endpoint string
}

// State é um retrato consistente do gerenciador, seguro para renderizar.
type State struct {
	Status   Status
	PID      int
	Since    time.Time
	Err      string
	ExitCode int
	Spec     StartSpec
}

func (s State) Uptime() time.Duration {
	if s.Since.IsZero() || !s.Status.Active() {
		return 0
	}
	return time.Since(s.Since).Truncate(time.Second)
}

var (
	ErrRunning    = errors.New("servidor já está rodando")
	ErrNotRunning = errors.New("servidor não está rodando")
	ansiRe        = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
)

type run struct {
	cmd      *exec.Cmd
	pgid     int
	spec     StartSpec
	done     chan struct{}
	stopping atomic.Bool
}

type Manager struct {
	mu       sync.Mutex
	capacity int
	lines    []Line
	seq      int
	dropped  int

	cur      *run
	status   Status
	pid      int
	since    time.Time
	errMsg   string
	exitCode int
	spec     StartSpec

	client *http.Client
}

func New(capacity int) *Manager {
	if capacity < 256 {
		capacity = 256
	}
	return &Manager{
		capacity: capacity,
		status:   StatusStopped,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return State{
		Status:   m.status,
		PID:      m.pid,
		Since:    m.since,
		Err:      m.errMsg,
		ExitCode: m.exitCode,
		Spec:     m.spec,
	}
}

// Since devolve as linhas com Seq > seq e o novo cursor.
func (m *Manager) Since(seq int) ([]Line, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.lines) == 0 {
		return nil, m.seq
	}
	first := m.lines[0].Seq
	start := seq + 1 - first
	if start < 0 {
		start = 0
	}
	if start >= len(m.lines) {
		return nil, m.seq
	}
	out := make([]Line, len(m.lines)-start)
	copy(out, m.lines[start:])
	return out, m.seq
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = nil
}

// Cursor devolve o seq atual, usado para pular o histórico já lido.
func (m *Manager) Cursor() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seq
}

func (m *Manager) Start(spec StartSpec) error {
	m.mu.Lock()
	if m.cur != nil {
		m.mu.Unlock()
		return ErrRunning
	}

	cmd := exec.Command(spec.Bin, spec.Args...)
	cmd.Env = spec.Env
	// grupo próprio: permite derrubar filhos junto com o servidor
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.mu.Unlock()
		return err
	}

	m.appendLocked(fmt.Sprintf("$ %s %s", spec.Bin, strings.Join(spec.Args, " ")), false)

	if err := cmd.Start(); err != nil {
		m.status = StatusFailed
		m.errMsg = err.Error()
		m.appendLocked("erro ao iniciar: "+err.Error(), true)
		m.mu.Unlock()
		return err
	}

	r := &run{cmd: cmd, pgid: cmd.Process.Pid, spec: spec, done: make(chan struct{})}
	m.cur = r
	m.status = StatusStarting
	m.pid = cmd.Process.Pid
	m.since = time.Now()
	m.errMsg = ""
	m.exitCode = 0
	m.spec = spec
	m.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go m.pump(&wg, stdout, false)
	go m.pump(&wg, stderr, true)

	if spec.Endpoint != "" {
		go m.pollHealth(r)
	}

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()

		m.mu.Lock()
		code := 0
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		m.exitCode = code
		m.cur = nil
		m.pid = 0
		switch {
		case r.stopping.Load():
			m.status = StatusStopped
			m.errMsg = ""
			m.appendLocked("-- servidor parado --", false)
		case code == 0 && waitErr == nil:
			m.status = StatusStopped
			m.appendLocked("-- servidor encerrou (código 0) --", false)
		default:
			m.status = StatusFailed
			m.errMsg = describeExit(code, waitErr)
			m.appendLocked("-- servidor encerrou: "+m.errMsg+" --", true)
		}
		m.mu.Unlock()
		close(r.done)
	}()

	return nil
}

// Stop pede o encerramento e devolve na hora; a transição final vem pelo State.
func (m *Manager) Stop(grace time.Duration) error {
	m.mu.Lock()
	r := m.cur
	if r == nil {
		m.mu.Unlock()
		return ErrNotRunning
	}
	if r.stopping.Swap(true) {
		m.mu.Unlock()
		return nil
	}
	m.status = StatusStopping
	m.appendLocked("-- SIGINT enviado --", false)
	m.mu.Unlock()

	go func() {
		_ = syscall.Kill(-r.pgid, syscall.SIGINT)
		select {
		case <-r.done:
			return
		case <-time.After(grace):
		}
		m.mu.Lock()
		m.appendLocked("-- sem resposta, enviando SIGKILL --", true)
		m.mu.Unlock()
		_ = syscall.Kill(-r.pgid, syscall.SIGKILL)
	}()
	return nil
}

// Restart para o processo atual (se houver) e sobe de novo com a spec dada.
func (m *Manager) Restart(spec StartSpec, grace time.Duration) error {
	m.mu.Lock()
	r := m.cur
	m.mu.Unlock()

	if r == nil {
		return m.Start(spec)
	}
	if err := m.Stop(grace); err != nil && !errors.Is(err, ErrNotRunning) {
		return err
	}
	go func() {
		<-r.done
		_ = m.Start(spec)
	}()
	return nil
}

// StopAndWait é usado na saída da TUI, quando precisamos garantir o encerramento.
func (m *Manager) StopAndWait(grace time.Duration) {
	m.mu.Lock()
	r := m.cur
	m.mu.Unlock()
	if r == nil {
		return
	}
	_ = m.Stop(grace)
	select {
	case <-r.done:
	case <-time.After(grace + 5*time.Second):
	}
}

func (m *Manager) pump(wg *sync.WaitGroup, r io.Reader, isErr bool) {
	defer wg.Done()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sc.Split(scanLinesCR)
	for sc.Scan() {
		text := ansiRe.ReplaceAllString(sc.Text(), "")
		m.mu.Lock()
		m.appendLocked(text, isErr)
		// cobre o formato antigo ("server is listening on http://...") e o atual
		// ("llama_server: listening on http://...")
		if m.status == StatusStarting && strings.Contains(text, "listening on http") {
			m.status = StatusRunning
		}
		m.mu.Unlock()
	}
}

func (m *Manager) pollHealth(r *run) {
	url := strings.TrimRight(r.spec.Endpoint, "/") + "/health"
	tick := time.NewTicker(700 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-tick.C:
		}
		resp, err := m.client.Get(url)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			m.mu.Lock()
			if m.status == StatusStarting {
				m.status = StatusRunning
				m.appendLocked("-- /health respondeu 200, servidor pronto --", false)
			}
			m.mu.Unlock()
			return
		}
	}
}

func (m *Manager) appendLocked(text string, isErr bool) {
	m.seq++
	m.lines = append(m.lines, Line{Seq: m.seq, Text: text, Err: isErr, At: time.Now()})
	if len(m.lines) > m.capacity {
		n := len(m.lines) - m.capacity
		m.dropped += n
		m.lines = append(m.lines[:0], m.lines[n:]...)
	}
}

func describeExit(code int, err error) string {
	if code >= 0 {
		return fmt.Sprintf("código %d", code)
	}
	if err != nil {
		return err.Error()
	}
	return "encerrado por sinal"
}

// scanLinesCR quebra em \n e também em \r, para não acumular barras de progresso.
func scanLinesCR(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\r' {
			if i+1 >= len(data) {
				if !atEOF {
					return 0, nil, nil // pode ser um \r\n cortado no meio
				}
				return i + 1, data[:i], nil
			}
			if data[i+1] == '\n' {
				return i + 2, data[:i], nil
			}
		}
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
