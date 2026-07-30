package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/juliosouzam/llamadeck/internal/catalog"
	"github.com/juliosouzam/llamadeck/internal/config"
	"github.com/juliosouzam/llamadeck/internal/server"
)

// key traduz uma tecla escrita como string para a KeyMsg equivalente.
func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "ctrl+r":
		return tea.KeyMsg{Type: tea.KeyCtrlR}
	case "ctrl+x":
		return tea.KeyMsg{Type: tea.KeyCtrlX}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeString(a *App, s string) {
	for _, r := range s {
		a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func press(a *App, keys ...string) {
	for _, k := range keys {
		a.Update(key(k))
	}
}

// newTestApp isola HOME e o cache de modelos num diretorio temporario.
func newTestApp(t *testing.T) (*App, *server.Manager) {
	t.Helper()
	home := t.TempDir()
	cache := filepath.Join(home, "cache")
	snap := filepath.Join(cache, "models--ggml-org--gemma-4-26B-A4B-it-GGUF", "snapshots", "aa")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"gemma-4-26B-A4B-it-Q4_0.gguf", "mtp-gemma-4-26B-A4B-it-Q4_0.gguf"} {
		if err := os.WriteFile(filepath.Join(snap, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("LLAMA_CACHE", cache)
	t.Setenv("LLAMA_MODELS", "")

	cfg := config.Default()
	cfg.Binary = "/bin/echo"
	mgr := server.New(256)
	a := New(cfg, mgr)
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return a, mgr
}

func TestModelSelectionAndMTPToggle(t *testing.T) {
	a, _ := newTestApp(t)

	if len(a.models) != 1 {
		t.Fatalf("esperava 1 modelo, veio %d: %+v", len(a.models), a.models)
	}
	if !a.models[0].HasMTP() {
		t.Fatal("sidecar MTP nao detectado")
	}

	press(a, "enter")
	if got := a.prof().Model.ID(); got != "ggml-org/gemma-4-26B-A4B-it-GGUF:Q4_0" {
		t.Fatalf("modelo selecionado = %q", got)
	}
	if !strings.Contains(strings.Join(a.prof().Args(), " "), "-hf ggml-org/") {
		t.Errorf("deveria subir via -hf: %v", a.prof().Args())
	}

	press(a, "p") // alterna para caminho local
	if !strings.Contains(strings.Join(a.prof().Args(), " "), "-m ") {
		t.Errorf("apos 'p' deveria usar -m: %v", a.prof().Args())
	}
	press(a, "p")

	press(a, "m") // liga MTP
	if v := a.prof().Params["spec-type"]; !v.Enabled || v.Value != "draft-mtp" {
		t.Errorf("spec-type = %+v", v)
	}
	press(a, "m") // desliga
	if v := a.prof().Params["spec-type"]; v.Value != "none" {
		t.Errorf("segundo 'm' deveria voltar para none, veio %+v", v)
	}

	press(a, "n") // --no-mmproj
	if !strings.Contains(strings.Join(a.prof().Args(), " "), "--no-mmproj") {
		t.Errorf("esperava --no-mmproj em %v", a.prof().Args())
	}

	view := a.View()
	if !strings.Contains(view, "llamadeck") || !strings.Contains(view, "Modelos") {
		t.Errorf("cabecalho ausente na view:\n%s", view)
	}
}

func TestParamsFilterToggleAndEdit(t *testing.T) {
	a, _ := newTestApp(t)
	press(a, "2") // aba de parametros

	press(a, "/")
	typeString(a, "ctx-size")
	press(a, "enter")

	if n := a.countParamRows(); n == 0 || n > 3 {
		t.Fatalf("filtro devolveu %d parametros, esperava poucos", n)
	}
	s, ok := a.currentSpec()
	if !ok || s.ID != "ctx-size" {
		t.Fatalf("cursor parou em %+v (ok=%v)", s, ok)
	}

	press(a, " ") // desliga (o default ja vem ligado)
	if a.prof().Params["ctx-size"].Enabled {
		t.Error("espaco deveria desligar o parametro")
	}
	press(a, " ")
	if !a.prof().Params["ctx-size"].Enabled {
		t.Error("espaco deveria religar o parametro")
	}

	press(a, "enter") // abre o editor de valor
	if a.mode != modeEdit {
		t.Fatal("enter num parametro numerico deveria abrir o editor")
	}
	a.input.SetValue("nao-e-numero")
	press(a, "enter")
	if a.mode != modeEdit {
		t.Error("valor invalido deveria manter o editor aberto")
	}
	a.input.SetValue("65536")
	press(a, "enter")
	if a.mode != modeNormal {
		t.Fatal("valor valido deveria fechar o editor")
	}
	if got := a.prof().Params["ctx-size"].Value; got != "65536" {
		t.Errorf("valor gravado = %q", got)
	}
	if !strings.Contains(strings.Join(a.prof().Args(), " "), "--ctx-size 65536") {
		t.Errorf("comando nao refletiu a edicao: %v", a.prof().Args())
	}

	press(a, "d") // volta ao default
	if _, ok := a.prof().Params["ctx-size"]; ok {
		t.Error("'d' deveria remover o override")
	}
}

func TestParamsEnumCycle(t *testing.T) {
	a, _ := newTestApp(t)
	press(a, "2", "/")
	typeString(a, "cache-type-k")
	press(a, "enter")

	before := a.paramValue(mustSpec(t, a)).Value
	press(a, "right")
	after := a.prof().Params["cache-type-k"].Value
	if before == after {
		t.Errorf("→ deveria ciclar o enum (%q)", after)
	}
	press(a, "left")
	if got := a.prof().Params["cache-type-k"].Value; got != before {
		t.Errorf("← deveria voltar para %q, veio %q", before, got)
	}
}

func TestStartStopFlow(t *testing.T) {
	a, mgr := newTestApp(t)
	press(a, "enter") // seleciona o modelo

	script := filepath.Join(t.TempDir(), "fake-server.sh")
	body := "#!/bin/sh\necho \"argumentos: $@\"\necho 'main: server is listening on http://127.0.0.1:49999'\nsleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfg.Binary = script
	a.prof().Params["port"] = config.ParamValue{Enabled: true, Value: "49999"}

	press(a, "ctrl+r")
	if a.tab != tabLogs {
		t.Error("subir o servidor deveria levar para a aba de logs")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.Update(tickMsg{})
		if a.state.Status == server.StatusRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if a.state.Status != server.StatusRunning {
		t.Fatalf("status = %v", a.state.Status)
	}

	view := a.View()
	if !strings.Contains(view, "rodando") {
		t.Errorf("view nao mostra o status:\n%s", view)
	}
	if !strings.Contains(view, "argumentos: -hf") {
		t.Errorf("logs do processo nao apareceram:\n%s", view)
	}

	// sair com o servidor ativo pede confirmacao
	press(a, "q")
	if a.mode != modeConfirmQuit {
		t.Fatal("sair com servidor ativo deveria pedir confirmacao")
	}
	press(a, "esc")
	if a.mode != modeNormal {
		t.Fatal("esc deveria cancelar a saida")
	}

	press(a, "ctrl+x")
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.Update(tickMsg{})
		if a.state.Status == server.StatusStopped {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if a.state.Status != server.StatusStopped {
		mgr.StopAndWait(time.Second)
		t.Fatalf("status apos ^x = %v", a.state.Status)
	}
}

func TestStartWithoutModelIsRefused(t *testing.T) {
	a, _ := newTestApp(t)
	press(a, "ctrl+r")
	if a.state.Status.Active() {
		t.Fatal("nao deveria subir sem modelo")
	}
	if !strings.Contains(a.toast, "selecione um modelo") {
		t.Errorf("toast = %q", a.toast)
	}
	if a.tab != tabModels {
		t.Error("deveria levar o usuario para a aba de modelos")
	}
}

func TestProfileSaveAndLoad(t *testing.T) {
	a, _ := newTestApp(t)
	press(a, "enter")
	a.prof().Params["ctx-size"] = config.ParamValue{Enabled: true, Value: "4096"}

	press(a, "3", "s")
	if a.mode != modeEdit {
		t.Fatal("'s' deveria pedir o nome do perfil")
	}
	a.input.SetValue("teste")
	press(a, "enter")

	if len(a.cfg.Profiles) != 1 || a.cfg.Profiles[0].Name != "teste" {
		t.Fatalf("perfis = %+v", a.cfg.Profiles)
	}

	a.prof().Params["ctx-size"] = config.ParamValue{Enabled: true, Value: "999"}
	press(a, "enter") // carrega o perfil selecionado
	if got := a.prof().Params["ctx-size"].Value; got != "4096" {
		t.Errorf("perfil carregado tem ctx-size = %q", got)
	}
}

func TestViewsRenderInEveryTab(t *testing.T) {
	a, _ := newTestApp(t)
	for _, tab := range []string{"1", "2", "3", "4"} {
		press(a, tab)
		out := a.View()
		if strings.TrimSpace(out) == "" {
			t.Fatalf("aba %s renderizou vazio", tab)
		}
		lines := strings.Split(out, "\n")
		// altura fixa: o rodape precisa ficar colado no fim da tela de 40 linhas
		if len(lines) != 39 {
			t.Errorf("aba %s renderizou %d linhas, esperava 39", tab, len(lines))
		}
		if !strings.Contains(lines[len(lines)-1], "subir/reiniciar") &&
			!strings.Contains(lines[len(lines)-1], a.toast) {
			t.Errorf("aba %s: ultima linha deveria ser o rodape, veio %q", tab, lines[len(lines)-1])
		}
		for i, line := range lines {
			if w := lineWidth(line); w > 120 {
				t.Errorf("aba %s linha %d estourou a largura (%d): %q", tab, i, w, line)
			}
		}
	}
}

func TestSmallTerminalDoesNotPanic(t *testing.T) {
	a, _ := newTestApp(t)
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 6}, {Width: 1, Height: 1}, {Width: 200, Height: 60}} {
		a.Update(size)
		for _, tab := range []string{"1", "2", "3", "4"} {
			press(a, tab)
			_ = a.View()
		}
	}
}

func mustSpec(t *testing.T, a *App) catalog.Spec {
	t.Helper()
	s, ok := a.currentSpec()
	if !ok {
		t.Fatal("cursor nao esta sobre um parametro")
	}
	return s
}

func lineWidth(s string) int { return ansi.StringWidth(s) }
