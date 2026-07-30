package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juliosouzam/llamadeck/internal/config"
	"github.com/juliosouzam/llamadeck/internal/models"
)

func (a *App) rescan() {
	a.models, a.modelWarns = models.Scan(a.cfg.ExtraDirs)
}

func (a *App) filteredModels() []models.Model {
	f := strings.ToLower(strings.TrimSpace(a.modelFilter))
	if f == "" {
		return a.models
	}
	out := make([]models.Model, 0, len(a.models))
	for _, m := range a.models {
		hay := strings.ToLower(m.ID() + " " + m.Path)
		if strings.Contains(hay, f) {
			out = append(out, m)
		}
	}
	return out
}

func (a *App) findModel(ref config.ModelRef) (models.Model, bool) {
	for _, m := range a.models {
		if ref.Repo != "" && m.Source == models.SourceHF && m.Repo == ref.Repo && m.Quant == ref.Quant {
			return m, true
		}
		if ref.Path != "" && m.Path == ref.Path {
			return m, true
		}
	}
	return models.Model{}, false
}

// syncModelCursor posiciona o cursor no modelo já gravado no perfil.
func (a *App) syncModelCursor() {
	ref := a.prof().Model
	if ref.Empty() {
		return
	}
	list := a.filteredModels()
	for i, m := range list {
		if m.ID() == ref.ID() || m.Path == ref.Path {
			a.modelCursor = i
			return
		}
	}
}

func (a *App) selectModel(m models.Model) {
	ref := config.ModelRef{Path: m.Path}
	if m.Source == models.SourceHF {
		ref.Repo = m.Repo
		ref.Quant = m.Quant
	} else {
		ref.UseLocalPath = true
	}
	a.prof().Model = ref
	a.notify("modelo selecionado: " + ref.ID())
}

func (a *App) keyModels(msg tea.KeyMsg) tea.Cmd {
	list := a.filteredModels()
	switch msg.String() {
	case "up", "k":
		if a.modelCursor > 0 {
			a.modelCursor--
		}
	case "down", "j":
		if a.modelCursor < len(list)-1 {
			a.modelCursor++
		}
	case "home", "g":
		a.modelCursor = 0
	case "end", "G":
		a.modelCursor = maxInt(0, len(list)-1)
	case "/":
		a.startFilter(a.modelFilter)
	case "r":
		a.rescan()
		a.notify("varredura refeita")
	case "enter", " ":
		if a.modelCursor < len(list) {
			a.selectModel(list[a.modelCursor])
		}
	case "p":
		ref := a.prof().Model
		if ref.Repo == "" {
			a.notify("modelo local já sobe por caminho (-m)")
			break
		}
		ref.UseLocalPath = !ref.UseLocalPath
		a.prof().Model = ref
		if ref.UseLocalPath {
			a.notify("usando -m (caminho local, sem rede e sem sidecar MTP automático)")
		} else {
			a.notify("usando -hf (resolve sidecar MTP do repo)")
		}
	case "m":
		a.toggleParam("spec-type", "draft-mtp", "none")
	case "n":
		a.toggleParam("mmproj-auto", "off", "on")
	}
	return nil
}

// toggleParam alterna um parâmetro entre dois valores, deixando-o sempre ativo.
func (a *App) toggleParam(id, on, off string) {
	p := a.prof()
	cur, ok := p.Params[id]
	next := on
	if ok && cur.Enabled && cur.Value == on {
		next = off
	}
	p.Params[id] = config.ParamValue{Enabled: true, Value: next}
	if s, ok := catalogLookup(id); ok {
		a.notify(s.Flag + " = " + next)
	}
}

func (a *App) viewModels(height int) string {
	list := a.filteredModels()
	lines := make([]string, 0, height)

	head := stySub.Render("origem: LLAMA_CACHE, LLAMA_MODELS e diretórios extras · " +
		strings.Join(shortRoots(a.cfg.ExtraDirs), ", "))
	lines = append(lines, a.clip(head))

	if a.modelFilter != "" {
		lines = append(lines, a.clip(styMuted.Render("filtro: ")+styValue.Render(a.modelFilter)))
	}
	if len(list) == 0 {
		lines = append(lines, "", a.clip(styWarn.Render("nenhum GGUF encontrado. baixe com: llama download -hf <org>/<repo>")))
		for _, w := range a.modelWarns {
			lines = append(lines, a.clip(styErr.Render(w)))
		}
		return strings.Join(lines, "\n")
	}

	if a.modelCursor >= len(list) {
		a.modelCursor = len(list) - 1
	}
	// reserva o rodapé do painel: linha em branco, caminho do gguf e sidecar MTP
	avail := maxInt(1, height-len(lines)-3)
	start, end := window(a.modelCursor, len(list), avail)
	ref := a.prof().Model

	for i := start; i < end; i++ {
		m := list[i]
		mark := "( )"
		if !ref.Empty() && (m.ID() == ref.ID() || (ref.Path != "" && m.Path == ref.Path)) {
			mark = styOn.Render("(•)")
		}
		cursor := "  "
		name := m.Title()
		if i == a.modelCursor {
			cursor = styCursor.Render("▸ ")
			name = styCursor.Render(name)
		}

		var badges []string
		if m.Quant != "" {
			badges = append(badges, styValue.Render(m.Quant))
		}
		badges = append(badges, styMuted.Render(m.SizeHuman()))
		if m.Shards > 1 {
			badges = append(badges, styMuted.Render("split"))
		}
		if m.HasMTP() {
			badges = append(badges, styBadge.Render("MTP"))
		}
		if m.HasMMProj() {
			badges = append(badges, styBadge.Render("mmproj"))
		}
		if m.Source == models.SourceLocal {
			badges = append(badges, stySub.Render("local"))
		}

		lines = append(lines, a.clip(cursor+mark+" "+name+"  "+strings.Join(badges, " ")))
	}

	if a.modelCursor < len(list) {
		sel := list[a.modelCursor]
		lines = append(lines, "", a.clip(stySub.Render(sel.Path)))
		if sel.HasMTP() {
			lines = append(lines, a.clip(stySub.Render("sidecar MTP: "+sel.MTPPath)))
		}
	}
	return strings.Join(lines, "\n")
}

func shortRoots(extra []string) []string {
	roots := models.Roots(extra)
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, r)
	}
	if len(out) == 0 {
		return []string{"(nenhum diretório encontrado)"}
	}
	return out
}
