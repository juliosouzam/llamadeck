package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juliocesar/llamadeck/internal/config"
)

func (a *App) keyProfiles(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if a.profCursor > 0 {
			a.profCursor--
		}
	case "down", "j":
		if a.profCursor < len(a.cfg.Profiles)-1 {
			a.profCursor++
		}
	case "enter":
		if a.profCursor < len(a.cfg.Profiles) {
			a.cfg.Current = a.cfg.Profiles[a.profCursor].Clone()
			a.buildParamRows()
			a.syncModelCursor()
			a.notify("perfil '" + a.cfg.Current.Name + "' carregado")
		}
	case "s":
		a.startEdit(editState{kind: editProfileName, label: "salvar perfil como"}, a.prof().Name)
	case "o":
		if a.profCursor < len(a.cfg.Profiles) {
			name := a.cfg.Profiles[a.profCursor].Name
			p := a.prof().Clone()
			p.Name = name
			a.cfg.UpsertProfile(p)
			a.notify("perfil '" + name + "' sobrescrito")
		}
	case "x":
		if a.profCursor < len(a.cfg.Profiles) {
			name := a.cfg.Profiles[a.profCursor].Name
			a.cfg.DeleteProfile(a.profCursor)
			if a.profCursor >= len(a.cfg.Profiles) {
				a.profCursor = maxInt(0, len(a.cfg.Profiles)-1)
			}
			a.notify("perfil '" + name + "' apagado")
		}
	case "b":
		a.startEdit(editState{kind: editBinary, label: "caminho do llama-server"}, a.cfg.Binary)
	case "D":
		a.startEdit(editState{kind: editExtraDirs, label: "diretorios extras (virgula)"}, strings.Join(a.cfg.ExtraDirs, ", "))
	case "e":
		a.startEdit(editState{kind: editEnv, label: "env do processo (K=V, virgula)"}, strings.Join(a.prof().EnvPairs(), ", "))
	case "E":
		a.cfg.KeepArgEnv = !a.cfg.KeepArgEnv
		if a.cfg.KeepArgEnv {
			a.notify("LLAMA_ARG_* do shell serao repassados ao servidor")
		} else {
			a.notify("LLAMA_ARG_* do shell serao removidos do servidor")
		}
	}
	return nil
}

func (a *App) viewProfiles(height int) string {
	lines := make([]string, 0, height)

	argEnv := styOK.Render("removidas")
	if a.cfg.KeepArgEnv {
		argEnv = styWarn.Render("repassadas")
	}
	env := a.prof().EnvPairs()
	envStr := stySub.Render("(nenhuma)")
	if len(env) > 0 {
		envStr = styValue.Render(strings.Join(env, " "))
	}

	lines = append(lines,
		a.clip(styGroup.Render("▪ Ajustes")),
		a.clip("  "+padTo(styMuted.Render("binario"), 24)+styValue.Render(a.cfg.Binary)+stySub.Render("   (b)")),
		a.clip("  "+padTo(styMuted.Render("dirs extras"), 24)+styValue.Render(orDash(strings.Join(a.cfg.ExtraDirs, ", ")))+stySub.Render("   (D)")),
		a.clip("  "+padTo(styMuted.Render("env do perfil"), 24)+envStr+stySub.Render("   (e)")),
		a.clip("  "+padTo(styMuted.Render("LLAMA_ARG_* do shell"), 24)+argEnv+stySub.Render("   (E)")),
		a.clip("  "+padTo(styMuted.Render("config"), 24)+stySub.Render(config.Path())),
		"",
		a.clip(styGroup.Render("▪ Perfis salvos")),
	)

	if len(a.cfg.Profiles) == 0 {
		lines = append(lines, a.clip(stySub.Render("  nenhum perfil salvo. use 's' para salvar o estado atual")))
		return strings.Join(lines, "\n")
	}

	avail := maxInt(1, height-len(lines))
	start, end := window(a.profCursor, len(a.cfg.Profiles), avail)
	for i := start; i < end; i++ {
		p := a.cfg.Profiles[i]
		cursor := "  "
		name := p.Name
		if i == a.profCursor {
			cursor = styCursor.Render("▸ ")
			name = styCursor.Render(name)
		}
		enabled := 0
		for _, v := range p.Params {
			if v.Enabled {
				enabled++
			}
		}
		info := stySub.Render(p.Model.ID() + "  ·  " + itoa(enabled) + " parametros")
		lines = append(lines, a.clip(cursor+padTo(name, 22)+info))
	}
	return strings.Join(lines, "\n")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
