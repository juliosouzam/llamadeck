package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type lipglossStyle = lipgloss.Style

func itoa(n int) string { return strconv.Itoa(n) }

// wrapText quebra o texto em linhas de no máximo w celulas, sem cortar runas.
func wrapText(s string, w int) []string {
	s = strings.ReplaceAll(s, "\t", "    ")
	if w <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= w {
		return []string{s}
	}
	out := make([]string, 0, len(runes)/w+1)
	for len(runes) > w {
		cut := w
		// tenta quebrar no último espaço da linha para não picar palavras
		for i := w; i > w*2/3 && i > 0; i-- {
			if runes[i-1] == ' ' {
				cut = i
				break
			}
		}
		out = append(out, strings.TrimRight(string(runes[:cut]), " "))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}
