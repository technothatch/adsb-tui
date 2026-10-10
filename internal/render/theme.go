// Package render turns state into terminal strings: the polar range
// display, the detail table, the framed panes and the status lines.
// Rendering is pure — given the same inputs it always produces the same
// output, which is what makes the whole TUI testable without a terminal.
package render

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Attr identifies the semantic role of a buffer cell or text run. The
// theme maps attributes to concrete styles, so rendering code never
// hard-codes a colour.
type Attr uint8

// The set of attributes used across the TUI.
const (
	// AttrNone is unstyled text (the buffer default).
	AttrNone Attr = iota
	// AttrBorder draws pane frames.
	AttrBorder
	// AttrTitle draws pane titles inside the frame.
	AttrTitle
	// AttrHeader draws table column headings.
	AttrHeader
	// AttrRing draws range rings and the crosshair.
	AttrRing
	// AttrSweep draws the rotating radar sweep.
	AttrSweep
	// AttrBlip draws an aircraft position marker.
	AttrBlip
	// AttrLabel draws the callsign/altitude label next to a blip.
	AttrLabel
	// AttrWarn highlights emergencies, warnings and the paused state.
	AttrWarn
	// AttrSel highlights the selected table row or blip.
	AttrSel
	// AttrDim draws secondary status text.
	AttrDim
	// AttrAccent draws the app name and the plot centre.
	AttrAccent
)

// Theme maps attributes to lipgloss styles. The zero Theme renders every
// attribute as plain text, so callers can always fall back safely.
type Theme struct {
	// Name is "dark" or "light".
	Name string
	// styles maps each Attr to its style; missing entries render plain.
	styles map[Attr]lipgloss.Style
}

// Style returns the style registered for an attribute (plain text when
// the theme has none).
func (t Theme) Style(a Attr) lipgloss.Style {
	return t.styles[a]
}

// NewTheme builds a theme from a name-to-style map.
func NewTheme(name string, styles map[Attr]lipgloss.Style) Theme {
	return Theme{Name: name, styles: styles}
}

// Dark is the default theme, tuned for a dark terminal background.
func Dark() Theme {
	return NewTheme("dark", map[Attr]lipgloss.Style{
		AttrBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("#45475a")),
		AttrTitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")),
		AttrHeader: lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Bold(true),
		AttrRing:   lipgloss.NewStyle().Foreground(lipgloss.Color("#313244")),
		AttrSweep:  lipgloss.NewStyle().Foreground(lipgloss.Color("#94e2d5")),
		AttrBlip:   lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Bold(true),
		AttrLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")),
		AttrWarn:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
		AttrSel:    lipgloss.NewStyle().Foreground(lipgloss.Color("#11111b")).Background(lipgloss.Color("#89b4fa")),
		AttrDim:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
		AttrAccent: lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")).Bold(true),
	})
}

// Light is the theme for terminals with a light background.
func Light() Theme {
	return NewTheme("light", map[Attr]lipgloss.Style{
		AttrBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af")),
		AttrTitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#1d4ed8")),
		AttrHeader: lipgloss.NewStyle().Foreground(lipgloss.Color("#111827")).Bold(true),
		AttrRing:   lipgloss.NewStyle().Foreground(lipgloss.Color("#d1d5db")),
		AttrSweep:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0d9488")),
		AttrBlip:   lipgloss.NewStyle().Foreground(lipgloss.Color("#15803d")).Bold(true),
		AttrLabel:  lipgloss.NewStyle().Foreground(lipgloss.Color("#a16207")),
		AttrWarn:   lipgloss.NewStyle().Foreground(lipgloss.Color("#b91c1c")).Bold(true),
		AttrSel:    lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#2563eb")),
		AttrDim:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")),
		AttrAccent: lipgloss.NewStyle().Foreground(lipgloss.Color("#c2410c")).Bold(true),
	})
}

// Next cycles a theme to the other one: dark → light → dark.
func Next(t Theme) Theme {
	if t.Name == "light" {
		return Dark()
	}
	return Light()
}

// ByName resolves a theme name; the empty name selects the default.
func ByName(name string) (Theme, error) {
	switch name {
	case "", "dark":
		return Dark(), nil
	case "light":
		return Light(), nil
	}
	return Theme{}, fmt.Errorf("unknown theme %q (want dark or light)", name)
}
