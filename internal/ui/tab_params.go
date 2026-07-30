package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/juliosouzam/llamadeck/internal/catalog"
	"github.com/juliosouzam/llamadeck/internal/config"
)

func catalogLookup(id string) (catalog.Spec, bool) { return catalog.Lookup(id) }

func (a *App) buildParamRows() {
	f := strings.ToLower(strings.TrimSpace(a.paramFilter))
	rows := make([]paramRow, 0, 128)
	for _, g := range catalog.Groups {
		matched := make([]catalog.Spec, 0, len(g.Specs))
		for _, s := range g.Specs {
			s.Group = g.Name
			if f == "" || specMatches(s, f) {
				matched = append(matched, s)
			}
		}
		if len(matched) == 0 {
			continue
		}
		rows = append(rows, paramRow{isGroup: true, group: g.Name})
		for _, s := range matched {
			rows = append(rows, paramRow{spec: s, group: g.Name})
		}
	}
	a.paramRows = rows
	a.snapParamCursor(1)
}

func specMatches(s catalog.Spec, f string) bool {
	hay := strings.ToLower(s.ID + " " + s.Flag + " " + s.Short + " " + s.Label + " " + s.Help + " " + s.Group)
	for _, term := range strings.Fields(f) {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}

// snapParamCursor move o cursor para a próxima linha que não seja cabeçalho.
func (a *App) snapParamCursor(dir int) {
	if len(a.paramRows) == 0 {
		a.paramCursor = 0
		return
	}
	if a.paramCursor < 0 {
		a.paramCursor = 0
		dir = 1
	}
	if a.paramCursor >= len(a.paramRows) {
		a.paramCursor = len(a.paramRows) - 1
		dir = -1
	}
	for a.paramRows[a.paramCursor].isGroup {
		next := a.paramCursor + dir
		if next < 0 || next >= len(a.paramRows) {
			dir = -dir
			next = a.paramCursor + dir
			if next < 0 || next >= len(a.paramRows) {
				return
			}
		}
		a.paramCursor = next
	}
}

func (a *App) currentSpec() (catalog.Spec, bool) {
	if a.paramCursor < 0 || a.paramCursor >= len(a.paramRows) {
		return catalog.Spec{}, false
	}
	row := a.paramRows[a.paramCursor]
	if row.isGroup {
		return catalog.Spec{}, false
	}
	return row.spec, true
}

func (a *App) paramValue(s catalog.Spec) config.ParamValue {
	if v, ok := a.prof().Params[s.ID]; ok {
		if v.Value == "" && s.Kind != catalog.KindFlag {
			v.Value = s.Default
		}
		return v
	}
	return config.ParamValue{Value: s.Default}
}

func (a *App) keyParams(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		a.paramCursor--
		a.snapParamCursor(-1)
	case "down", "j":
		a.paramCursor++
		a.snapParamCursor(1)
	case "pgup", "ctrl+u":
		a.paramCursor -= 10
		a.snapParamCursor(-1)
	case "pgdown", "ctrl+d":
		a.paramCursor += 10
		a.snapParamCursor(1)
	case "home":
		a.paramCursor = 0
		a.snapParamCursor(1)
	case "end":
		a.paramCursor = len(a.paramRows) - 1
		a.snapParamCursor(-1)
	case "/":
		a.startFilter(a.paramFilter)
	case " ":
		if s, ok := a.currentSpec(); ok {
			v := a.paramValue(s)
			v.Enabled = !v.Enabled
			if v.Enabled && v.Value == "" {
				v.Value = s.Default
			}
			a.prof().Params[s.ID] = v
		}
	case "enter":
		s, ok := a.currentSpec()
		if !ok {
			break
		}
		v := a.paramValue(s)
		switch s.Kind {
		case catalog.KindFlag:
			v.Enabled = !v.Enabled
			a.prof().Params[s.ID] = v
		case catalog.KindToggle, catalog.KindEnum:
			v.Enabled = true
			v.Value = s.Next(v.Value)
			a.prof().Params[s.ID] = v
		default:
			a.startEdit(editState{kind: editParam, id: s.ID, label: s.Flag}, v.Value)
		}
	case "e":
		if s, ok := a.currentSpec(); ok {
			a.startEdit(editState{kind: editParam, id: s.ID, label: s.Flag}, a.paramValue(s).Value)
		}
	case "right", "l":
		if s, ok := a.currentSpec(); ok && (s.Kind == catalog.KindEnum || s.Kind == catalog.KindToggle) {
			v := a.paramValue(s)
			v.Enabled = true
			v.Value = s.Next(v.Value)
			a.prof().Params[s.ID] = v
		}
	case "left", "h":
		if s, ok := a.currentSpec(); ok && (s.Kind == catalog.KindEnum || s.Kind == catalog.KindToggle) {
			v := a.paramValue(s)
			v.Enabled = true
			v.Value = s.Prev(v.Value)
			a.prof().Params[s.ID] = v
		}
	case "d":
		if s, ok := a.currentSpec(); ok {
			delete(a.prof().Params, s.ID)
			a.notify(s.Flag + " voltou ao default")
		}
	case "X":
		n := 0
		for _, row := range a.paramRows {
			if row.isGroup {
				continue
			}
			if v, ok := a.prof().Params[row.spec.ID]; ok && v.Enabled {
				delete(a.prof().Params, row.spec.ID)
				n++
			}
		}
		a.notify(strings.TrimSpace(itoa(n) + " parâmetros desativados"))
	}
	return nil
}

func (a *App) viewParams(height int) string {
	var head []string
	cmd := config.Shell(a.cfg.Binary, a.prof().Args())
	preview := wrapText(cmd, maxInt(20, a.width-2))
	if len(preview) > 3 {
		preview = append(preview[:3:3], stySub.Render("... (+"+itoa(len(preview)-3)+" linhas)"))
	}
	head = append(head, styMuted.Render("comando"))
	for _, l := range preview {
		head = append(head, "  "+styValue.Render(l))
	}
	head = append(head, rule(a.width))
	if a.paramFilter != "" {
		head = append(head, a.clip(styMuted.Render("filtro: ")+styValue.Render(a.paramFilter)+
			stySub.Render("  ("+itoa(a.countParamRows())+" parâmetros)")))
	}

	var foot []string
	if s, ok := a.currentSpec(); ok {
		foot = append(foot, rule(a.width), a.clip(styMuted.Render(s.Display()+"  ")+stySub.Render(s.Help)))
	}

	avail := height - len(head) - len(foot)
	if avail < 1 {
		avail = 1
	}
	start, end := window(a.paramCursor, len(a.paramRows), avail)

	flagW := 30
	valW := 16
	if a.width < 90 {
		flagW = 22
		valW = 12
	}

	body := make([]string, 0, avail)
	for i := start; i < end; i++ {
		row := a.paramRows[i]
		if row.isGroup {
			body = append(body, a.clip(styGroup.Render("▪ "+row.group)))
			continue
		}
		s := row.spec
		v := a.paramValue(s)

		box := "[ ]"
		if v.Enabled {
			box = styOn.Render("[x]")
		}
		cursor := "  "
		if i == a.paramCursor {
			cursor = styCursor.Render("▸ ")
		}

		val := v.Value
		if s.Kind == catalog.KindFlag {
			val = "-"
		}
		if val == "" {
			val = stySub.Render("(vazio)")
		} else if v.Enabled {
			val = styValue.Render(val)
		} else {
			val = stySub.Render(val)
		}

		name := s.Display()
		if !v.Enabled {
			name = stySub.Render(name)
		}
		line := cursor + box + " " + padTo(name, flagW) + " " + padTo(val, valW) + " " + stySub.Render(s.Label)
		body = append(body, a.clip(line))
	}

	out := append(head, body...)
	return strings.Join(append(out, foot...), "\n")
}

func (a *App) countParamRows() int {
	n := 0
	for _, r := range a.paramRows {
		if !r.isGroup {
			n++
		}
	}
	return n
}
