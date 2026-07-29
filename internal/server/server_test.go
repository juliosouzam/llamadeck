package server

import (
	"os"
	"strings"
	"testing"
	"time"
)

func waitFor(t *testing.T, m *Manager, want Status, timeout time.Duration) State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st := m.State()
		if st.Status == want {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	st := m.State()
	lines, _ := m.Since(0)
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text + "\n")
	}
	t.Fatalf("status %v nao chegou (atual %v)\nlogs:\n%s", want, st.Status, b.String())
	return st
}

func TestStartCaptureAndStop(t *testing.T) {
	m := New(512)
	spec := StartSpec{
		Bin:  "/bin/sh",
		Args: []string{"-c", "echo carregando; echo 'main: server is listening on http://127.0.0.1:8080'; sleep 30"},
		Env:  os.Environ(),
	}
	if err := m.Start(spec); err != nil {
		t.Fatal(err)
	}
	st := waitFor(t, m, StatusRunning, 5*time.Second)
	if st.PID == 0 {
		t.Error("pid nao registrado")
	}

	lines, seq := m.Since(0)
	var joined string
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if !strings.Contains(joined, "carregando") {
		t.Errorf("stdout nao capturado: %s", joined)
	}
	if !strings.HasPrefix(lines[0].Text, "$ /bin/sh") {
		t.Errorf("primeira linha deveria ser o comando, veio %q", lines[0].Text)
	}
	if more, _ := m.Since(seq); len(more) != 0 {
		t.Errorf("Since deveria estar drenado, veio %d linhas", len(more))
	}

	if err := m.Start(spec); err != ErrRunning {
		t.Errorf("segundo Start deveria falhar com ErrRunning, veio %v", err)
	}

	if err := m.Stop(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, StatusStopped, 5*time.Second)

	if err := m.Stop(time.Second); err != ErrNotRunning {
		t.Errorf("Stop com servidor parado = %v", err)
	}
}

func TestFailedExitIsReported(t *testing.T) {
	m := New(256)
	err := m.Start(StartSpec{Bin: "/bin/sh", Args: []string{"-c", "echo boom >&2; exit 3"}, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	st := waitFor(t, m, StatusFailed, 5*time.Second)
	if st.ExitCode != 3 {
		t.Errorf("exit code = %d", st.ExitCode)
	}
	lines, _ := m.Since(0)
	var joined string
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if !strings.Contains(joined, "boom") {
		t.Errorf("stderr nao capturado: %s", joined)
	}
}

func TestSIGKILLFallback(t *testing.T) {
	m := New(256)
	// ignora SIGINT: forca o caminho do SIGKILL
	err := m.Start(StartSpec{Bin: "/bin/sh", Args: []string{"-c", "trap '' INT; sleep 30"}, Env: os.Environ()})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, StatusStarting, 2*time.Second)
	if err := m.Stop(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, StatusStopped, 5*time.Second)
}

func TestRestartReusesSpec(t *testing.T) {
	m := New(256)
	spec := StartSpec{Bin: "/bin/sh", Args: []string{"-c", "echo primeira; sleep 30"}, Env: os.Environ()}
	if err := m.Start(spec); err != nil {
		t.Fatal(err)
	}
	waitFor(t, m, StatusStarting, 2*time.Second)

	spec2 := StartSpec{Bin: "/bin/sh", Args: []string{"-c", "echo segunda; sleep 30"}, Env: os.Environ()}
	if err := m.Restart(spec2, time.Second); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		lines, _ := m.Since(0)
		for _, l := range lines {
			if strings.Contains(l.Text, "segunda") {
				m.StopAndWait(time.Second)
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	m.StopAndWait(time.Second)
	t.Fatal("o restart nao subiu a nova spec")
}

func TestScanLinesCRSplitsProgress(t *testing.T) {
	adv, tok, err := scanLinesCR([]byte("10%\r20%\n"), false)
	if err != nil || adv != 4 || string(tok) != "10%" {
		t.Fatalf("adv=%d tok=%q err=%v", adv, tok, err)
	}
	// \r no fim do buffer: precisa de mais dados para saber se e \r\n
	if adv, tok, _ := scanLinesCR([]byte("abc\r"), false); adv != 0 || tok != nil {
		t.Fatalf("esperava pedir mais dados, veio adv=%d tok=%q", adv, tok)
	}
	if adv, tok, _ := scanLinesCR([]byte("abc\r\n"), false); adv != 5 || string(tok) != "abc" {
		t.Fatalf("crlf: adv=%d tok=%q", adv, tok)
	}
}
