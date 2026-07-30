// Package ui implementa a TUI em bubbletea.
package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/juliosouzam/llamadeck/internal/catalog"
	"github.com/juliosouzam/llamadeck/internal/config"
	"github.com/juliosouzam/llamadeck/internal/models"
	"github.com/juliosouzam/llamadeck/internal/server"
)

const (
	headerHeight = 4
	footerHeight = 2
	stopGrace    = 10 * time.Second
)

type tabID int

const (
	tabModels tabID = iota
	tabParams
	tabProfiles
	tabLogs
	tabCount
)

var tabNames = [tabCount]string{"Modelos", "Parâmetros", "Perfis", "Logs"}

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeEdit
	modeConfirmQuit
)

type editKind int

const (
	editParam editKind = iota
	editProfileName
	editBinary
	editExtraDirs
	editEnv
)

type editState struct {
	kind  editKind
	id    string
	label string
}

type paramRow struct {
	isGroup bool
	group   string
	spec    catalog.Spec
}

type tickMsg struct{}

// App é o modelo raiz da TUI.
type App struct {
	cfg *config.Config
	mgr *server.Manager

	width, height int
	tab           tabID
	mode          mode

	models      []models.Model
	modelWarns  []string
	modelCursor int
	modelFilter string

	paramRows   []paramRow
	paramCursor int
	paramFilter string

	profCursor int

	logLines  []server.Line
	logSeq    int
	logFilter string
	follow    bool
	logDirty  bool
	vp        viewport.Model

	input textinput.Model
	edit  editState

	state      server.State
	toast      string
	toastUntil time.Time
	quitStop   bool
}

// New monta a aplicação com a configuração já carregada do disco.
func New(cfg *config.Config, mgr *server.Manager) *App {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 4096

	a := &App{
		cfg:    cfg,
		mgr:    mgr,
		follow: true,
		input:  in,
		vp:     viewport.New(80, 20),
	}
	if a.cfg.Current.Params == nil {
		a.cfg.Current = config.DefaultProfile()
	}
	a.rescan()
	a.buildParamRows()
	a.syncModelCursor()
	return a
}

func (a *App) prof() *config.Profile { return &a.cfg.Current }

// StopOnExit informa se o usuário pediu para derrubar o servidor ao sair.
func (a *App) StopOnExit() bool { return a.quitStop }

func (a *App) Init() tea.Cmd {
	return tea.Batch(tickCmd(700*time.Millisecond), textinput.Blink)
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{} })
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.vp.Width = maxInt(20, a.width-2)
		a.vp.Height = maxInt(3, a.contentHeight()-2)
		a.logDirty = true

	case tickMsg:
		a.state = a.mgr.State()
		if lines, seq := a.mgr.Since(a.logSeq); len(lines) > 0 {
			a.logLines = append(a.logLines, lines...)
			a.logSeq = seq
			a.trimLogs()
			a.logDirty = true
		}
		interval := 700 * time.Millisecond
		if a.state.Status.Active() {
			interval = 120 * time.Millisecond
		}
		if a.logDirty {
			a.rebuildLogs()
		}
		return a, tickCmd(interval)

	case tea.KeyMsg:
		cmd := a.handleKey(msg)
		if a.logDirty {
			a.rebuildLogs()
		}
		return a, cmd
	}

	if a.tab == tabLogs && a.mode == modeNormal {
		var cmd tea.Cmd
		a.vp, cmd = a.vp.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch a.mode {
	case modeConfirmQuit:
		switch msg.String() {
		case "s", "S", "y", "Y":
			a.quitStop = true
			return tea.Quit
		case "n", "N":
			a.quitStop = false
			return tea.Quit
		case "esc", "q", "ctrl+c":
			a.mode = modeNormal
		}
		return nil

	case modeFilter, modeEdit:
		switch msg.String() {
		case "esc":
			a.mode = modeNormal
			a.input.Blur()
			if a.tab == tabLogs {
				a.logDirty = true
			}
			return nil
		case "enter":
			return a.commitInput()
		}
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		if a.mode == modeFilter {
			a.applyFilter(a.input.Value())
		}
		return cmd
	}

	switch msg.String() {
	case "ctrl+c":
		if a.state.Status.Active() {
			a.mode = modeConfirmQuit
			return nil
		}
		return tea.Quit
	case "q":
		if a.state.Status.Active() {
			a.mode = modeConfirmQuit
			return nil
		}
		return tea.Quit
	case "tab":
		a.tab = (a.tab + 1) % tabCount
		return nil
	case "shift+tab":
		a.tab = (a.tab + tabCount - 1) % tabCount
		return nil
	case "1", "2", "3", "4":
		n, _ := strconv.Atoi(msg.String())
		a.tab = tabID(n - 1)
		return nil
	case "ctrl+r":
		a.startOrRestart()
		return nil
	case "ctrl+x":
		a.stop()
		return nil
	case "ctrl+s":
		a.saveConfig()
		return nil
	}

	switch a.tab {
	case tabModels:
		return a.keyModels(msg)
	case tabParams:
		return a.keyParams(msg)
	case tabProfiles:
		return a.keyProfiles(msg)
	case tabLogs:
		return a.keyLogs(msg)
	}
	return nil
}

func (a *App) View() string {
	if a.width == 0 {
		return "carregando..."
	}
	header := a.renderHeader()
	footer := a.renderFooter()

	h := a.contentHeight()
	var content string
	switch a.tab {
	case tabModels:
		content = a.viewModels(h)
	case tabParams:
		content = a.viewParams(h)
	case tabProfiles:
		content = a.viewProfiles(h)
	case tabLogs:
		content = a.viewLogs(h)
	}
	return header + "\n" + fit(content, h) + "\n" + footer
}

// fit ajusta o bloco de conteúdo para exatamente height linhas, para que o
// rodapé fique sempre colado no fim da tela.
func fit(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	return content + strings.Repeat("\n", height-len(lines))
}

func (a *App) contentHeight() int {
	return maxInt(3, a.height-headerHeight-footerHeight-1)
}

// ---------- cabeçalho e rodapé ----------

func (a *App) renderHeader() string {
	st := a.state
	var badge string
	switch st.Status {
	case server.StatusRunning:
		badge = styOK.Render("● rodando")
	case server.StatusStarting:
		badge = styWarn.Render("◐ subindo")
	case server.StatusStopping:
		badge = styWarn.Render("◑ parando")
	case server.StatusFailed:
		badge = styErr.Render("✗ falhou")
	default:
		badge = stySub.Render("○ parado")
	}

	line1 := []string{styTitle.Render("llamadeck"), badge}
	if st.PID > 0 {
		line1 = append(line1, styMuted.Render(fmt.Sprintf("pid %d", st.PID)))
	}
	if d := st.Uptime(); d > 0 {
		line1 = append(line1, styMuted.Render("up "+fmtDuration(d)))
	}
	if ep := a.prof().Endpoint(); ep != "" && st.Status.Active() {
		line1 = append(line1, styValue.Render(ep))
	}
	if st.Status == server.StatusFailed && st.Err != "" {
		line1 = append(line1, styErr.Render(st.Err))
	}

	mref := a.prof().Model
	var line2 []string
	if mref.Empty() {
		line2 = append(line2, styMuted.Render("modelo"), styWarn.Render("nenhum selecionado (aba 1)"))
	} else {
		line2 = append(line2, styMuted.Render("modelo"), styValue.Render(mref.ID()))
		if m, ok := a.findModel(mref); ok {
			line2 = append(line2, styMuted.Render(m.SizeHuman()))
			if m.HasMTP() {
				line2 = append(line2, styBadge.Render("MTP"))
			}
			if m.HasMMProj() {
				line2 = append(line2, styBadge.Render("mmproj"))
			}
		}
		if mref.UseLocalPath || mref.Repo == "" {
			line2 = append(line2, styMuted.Render("via -m"))
		} else {
			line2 = append(line2, styMuted.Render("via -hf"))
		}
		if v, ok := a.prof().Params["spec-type"]; ok && v.Enabled && v.Value != "none" {
			line2 = append(line2, styOn.Render("spec="+v.Value))
		}
	}

	return strings.Join([]string{
		a.clip(strings.Join(line1, "  ")),
		a.clip(strings.Join(line2, "  ")),
		a.renderTabs(),
		rule(a.width),
	}, "\n")
}

func (a *App) renderTabs() string {
	parts := make([]string, 0, tabCount)
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if tabID(i) == a.tab {
			parts = append(parts, styTabActive.Render(label))
		} else {
			parts = append(parts, styTabInactive.Render(label))
		}
	}
	return a.clip(strings.Join(parts, stySub.Render("│")))
}

func (a *App) renderFooter() string {
	if a.mode == modeConfirmQuit {
		return rule(a.width) + "\n" + a.clip(styWarn.Render("servidor ativo -- (s) parar e sair   (n) deixar rodando   (esc) cancelar"))
	}
	if a.mode == modeEdit || a.mode == modeFilter {
		label := "filtro"
		if a.mode == modeEdit {
			label = a.edit.label
		}
		return rule(a.width) + "\n" + a.clip(styTitle.Render(label+": ")+a.input.View()+stySub.Render("   enter confirma · esc cancela"))
	}
	if a.toast != "" && time.Now().Before(a.toastUntil) {
		return rule(a.width) + "\n" + a.clip(styToast.Render(a.toast))
	}

	var keys string
	switch a.tab {
	case tabModels:
		keys = "enter selecionar · p -m/-hf · m MTP · n mmproj · r rescan · / filtrar"
	case tabParams:
		keys = "espaço liga/desliga · enter editar · ←/→ ciclar · d default · X desliga visíveis · / filtrar"
	case tabProfiles:
		keys = "enter carregar · s salvar como · o sobrescrever · x apagar · b binário · D dirs · e env"
	case tabLogs:
		keys = "f follow · g/G topo/fim · ^u/^d meia página · c limpar · / filtrar"
	}
	global := "  ·  ^r subir/reiniciar · ^x parar · ^s salvar config · tab trocar · q sair"
	return rule(a.width) + "\n" + a.clip(stySub.Render(keys+global))
}

// ---------- ações ----------

func (a *App) startOrRestart() {
	p := a.prof()
	if p.Model.Empty() {
		a.notify("selecione um modelo na aba 1 antes de subir")
		a.tab = tabModels
		return
	}
	if a.cfg.Binary == "" {
		a.notify("binário do llama-server não configurado (aba 3, tecla b)")
		return
	}
	spec := server.StartSpec{
		Bin:      a.cfg.Binary,
		Args:     p.Args(),
		Env:      a.childEnv(),
		Endpoint: p.Endpoint(),
	}
	st := a.mgr.State()
	var err error
	if st.Status.Active() {
		err = a.mgr.Restart(spec, stopGrace)
		a.notify("reiniciando o servidor")
	} else {
		err = a.mgr.Start(spec)
		a.notify("subindo o servidor")
	}
	if err != nil {
		a.notify("erro: " + err.Error())
		return
	}
	a.tab = tabLogs
	a.follow = true
	a.state = a.mgr.State()
}

func (a *App) stop() {
	if err := a.mgr.Stop(stopGrace); err != nil {
		a.notify(err.Error())
		return
	}
	a.notify("parando o servidor")
	a.state = a.mgr.State()
}

func (a *App) saveConfig() {
	if err := a.cfg.Save(); err != nil {
		a.notify("erro ao salvar: " + err.Error())
		return
	}
	a.notify("config salva em " + config.Path())
}

// childEnv monta o ambiente do processo filho. Por padrão remove as LLAMA_ARG_*
// para que apenas os parâmetros marcados na TUI valham.
func (a *App) childEnv() []string {
	out := make([]string, 0, 64)
	for _, kv := range os.Environ() {
		if !a.cfg.KeepArgEnv && strings.HasPrefix(kv, "LLAMA_ARG_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, a.prof().EnvPairs()...)
}

func (a *App) notify(msg string) {
	a.toast = msg
	a.toastUntil = time.Now().Add(4 * time.Second)
}

// ---------- entrada de texto ----------

func (a *App) startFilter(current string) {
	a.mode = modeFilter
	a.input.SetValue(current)
	a.input.CursorEnd()
	a.input.Width = maxInt(10, a.width-20)
	a.input.Focus()
}

func (a *App) startEdit(e editState, current string) {
	a.mode = modeEdit
	a.edit = e
	a.input.SetValue(current)
	a.input.CursorEnd()
	a.input.Width = maxInt(10, a.width-20)
	a.input.Focus()
}

func (a *App) applyFilter(v string) {
	switch a.tab {
	case tabModels:
		a.modelFilter = v
		a.modelCursor = 0
	case tabParams:
		a.paramFilter = v
		a.buildParamRows()
	case tabLogs:
		a.logFilter = v
		a.logDirty = true
	}
}

func (a *App) commitInput() tea.Cmd {
	v := strings.TrimSpace(a.input.Value())
	if a.mode == modeFilter {
		a.applyFilter(a.input.Value())
		a.mode = modeNormal
		a.input.Blur()
		return nil
	}

	switch a.edit.kind {
	case editParam:
		s, ok := catalog.Lookup(a.edit.id)
		if !ok {
			break
		}
		if err := validate(s, v); err != nil {
			a.notify(err.Error())
			return nil
		}
		a.prof().Params[s.ID] = config.ParamValue{Enabled: true, Value: v}
	case editProfileName:
		if v == "" {
			a.notify("nome vazio")
			return nil
		}
		p := a.prof().Clone()
		p.Name = v
		a.cfg.UpsertProfile(p)
		a.notify("perfil '" + v + "' salvo")
	case editBinary:
		a.cfg.Binary = v
		a.notify("binário: " + v)
	case editExtraDirs:
		a.cfg.ExtraDirs = splitList(v)
		a.rescan()
		a.notify("diretórios atualizados")
	case editEnv:
		a.setEnvFromString(v)
		a.notify("variáveis de ambiente atualizadas")
	}

	a.mode = modeNormal
	a.input.Blur()
	return nil
}

func validate(s catalog.Spec, v string) error {
	if v == "" {
		return nil
	}
	switch s.Kind {
	case catalog.KindInt:
		if _, err := strconv.Atoi(v); err != nil {
			return fmt.Errorf("%s espera um inteiro", s.Flag)
		}
	case catalog.KindFloat:
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return fmt.Errorf("%s espera um número", s.Flag)
		}
	}
	return nil
}

func (a *App) setEnvFromString(v string) {
	env := map[string]string{}
	for _, part := range splitList(v) {
		k, val, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(k) == "" {
			continue
		}
		env[strings.TrimSpace(k)] = val
	}
	if len(env) == 0 {
		a.prof().Env = nil
		return
	}
	a.prof().Env = env
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- utilidades ----------

func (a *App) clip(s string) string {
	if a.width <= 0 {
		return s
	}
	return ansi.Truncate(s, a.width, "…")
}

func clipTo(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

func padTo(s string, w int) string {
	d := w - ansi.StringWidth(s)
	if d <= 0 {
		return clipTo(s, w)
	}
	return s + strings.Repeat(" ", d)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// window devolve a janela visível de uma lista, mantendo o cursor a vista.
func window(cursor, total, height int) (int, int) {
	if height <= 0 || total == 0 {
		return 0, 0
	}
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}
