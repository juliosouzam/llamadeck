package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const maxLogLines = 5000

func (a *App) trimLogs() {
	if len(a.logLines) > maxLogLines {
		n := len(a.logLines) - maxLogLines
		a.logLines = append(a.logLines[:0], a.logLines[n:]...)
	}
}

func (a *App) keyLogs(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "/":
		a.startFilter(a.logFilter)
	case "f":
		a.follow = !a.follow
		if a.follow {
			a.vp.GotoBottom()
			a.notify("follow ligado")
		} else {
			a.notify("follow desligado")
		}
	case "g", "home":
		a.follow = false
		a.vp.GotoTop()
	case "G", "end":
		a.follow = true
		a.vp.GotoBottom()
	case "c":
		a.mgr.Clear()
		a.logLines = nil
		a.logSeq = a.mgr.Cursor()
		a.logDirty = true
		a.notify("logs limpos")
	case "up", "k":
		a.follow = false
		a.vp.LineUp(1)
	case "down", "j":
		a.vp.LineDown(1)
		if a.vp.AtBottom() {
			a.follow = true
		}
	case "pgup":
		a.follow = false
		a.vp.ViewUp()
	case "pgdown":
		a.vp.ViewDown()
	case "ctrl+u":
		a.follow = false
		a.vp.HalfViewUp()
	case "ctrl+d":
		a.vp.HalfViewDown()
	}
	return nil
}

func (a *App) rebuildLogs() {
	a.logDirty = false
	w := maxInt(20, a.width-1)
	filter := strings.ToLower(strings.TrimSpace(a.logFilter))

	var b strings.Builder
	b.Grow(len(a.logLines) * 64)
	for _, ln := range a.logLines {
		if filter != "" && !strings.Contains(strings.ToLower(ln.Text), filter) {
			continue
		}
		style := logStyleFor(ln.Text)
		for _, part := range wrapText(ln.Text, w) {
			if style != nil {
				b.WriteString(style.Render(part))
			} else {
				b.WriteString(part)
			}
			b.WriteByte('\n')
		}
	}
	a.vp.SetContent(strings.TrimRight(b.String(), "\n"))
	if a.follow {
		a.vp.GotoBottom()
	}
}

func (a *App) viewLogs(height int) string {
	a.vp.Width = maxInt(20, a.width)
	a.vp.Height = maxInt(1, height-1)

	status := []string{}
	if a.follow {
		status = append(status, styOK.Render("follow"))
	} else {
		status = append(status, stySub.Render("follow off"))
	}
	status = append(status, styMuted.Render(itoa(len(a.logLines))+" linhas"))
	if a.logFilter != "" {
		status = append(status, styMuted.Render("filtro: ")+styValue.Render(a.logFilter))
	}
	if st := a.state; st.Status.String() != "" && len(st.Spec.Args) > 0 {
		status = append(status, stySub.Render(strings.Join(st.Spec.Args, " ")))
	}

	return a.clip(strings.Join(status, "  ·  ")) + "\n" + a.vp.View()
}

func logStyleFor(s string) *lipglossStyle {
	low := strings.ToLower(s)
	switch {
	case strings.Contains(low, "error") || strings.Contains(low, "failed") || strings.Contains(low, "erro:") ||
		strings.Contains(low, "cannot ") || strings.Contains(low, "couldn't"):
		return &styErr
	case strings.Contains(low, "warn"):
		return &styWarn
	case strings.Contains(low, "listening on http") || strings.Contains(low, "servidor pronto"):
		return &styOK
	case strings.HasPrefix(s, "$ ") || strings.HasPrefix(s, "-- "):
		return &styMuted
	}
	return nil
}
