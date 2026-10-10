package model

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/technothatch/adsb-tui/internal/render"
)

// handleKey dispatches a key press while the filter input is idle.
// While typing (m.filtering) every printable key goes to the draft
// instead, so "q" inside a filter never quits the program.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.filtering {
		return m.filterKey(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		// Esc closes help first, then clears the filter; quitting is
		// the job of "q" so an accidental press cannot lose the view.
		if m.help {
			m.help = false
			m.clampScroll()
			return m, nil
		}
		if m.filter != "" {
			m.filter = ""
			m.rebuild()
		}
		return m, nil
	case "t":
		m.pane = m.pane.next(m.hasRecv)
		m.clampScroll()
		return m, nil
	case "h":
		m.help = !m.help
		m.clampScroll()
		return m, nil
	case "p":
		m.paused = !m.paused
		if m.paused {
			return m, nil
		}
		return m, m.scheduleRefresh()
	case "d":
		m.theme = render.Next(m.theme)
		return m, nil
	case "/":
		m.filtering = true
		m.draft = m.filter
		return m, nil
	case "1":
		m.sort = SortDistance
		m.rebuild()
		return m, nil
	case "2":
		m.sort = SortCallsign
		m.rebuild()
		return m, nil
	case "3":
		m.sort = SortAltitude
		m.rebuild()
		return m, nil
	case "up", "k":
		m.cursor--
	case "down", "j":
		m.cursor++
	case "pgup":
		m.cursor -= m.tableRowsHeight()
	case "pgdn", "pgdown":
		m.cursor += m.tableRowsHeight()
	case "home", "g":
		m.cursor = 0
	case "end":
		m.cursor = len(m.entries) - 1
	default:
		return m, nil
	}
	m.clampScroll()
	return m, nil
}

// filterKey handles a key press while the "/" filter input is open.
func (m Model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.filtering = false
		m.filter = strings.TrimSpace(m.draft)
		m.draft = ""
		m.cursor, m.offset = 0, 0
		m.rebuild()
	case "esc":
		m.filtering = false
		m.draft = ""
	case "backspace":
		if r := []rune(m.draft); len(r) > 0 {
			m.draft = string(r[:len(r)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.draft += string(msg.Runes)
		}
	}
	return m, nil
}
