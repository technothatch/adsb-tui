package render

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// ansiWidth is the visible cell width of s, ignoring escape sequences.
func ansiWidth(s string) int { return ansi.StringWidth(s) }

// Fit clips s to at most w cells of visible width. ANSI escape
// sequences are preserved (and do not count towards the width) until
// they fall beyond the cut, so styling survives truncation.
func Fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}

// PadRight returns s clipped or padded with spaces to exactly w cells.
func PadRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Fit(s, w)
	if n := ansi.StringWidth(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// PadLeft returns s clipped or padded to exactly w cells with the text
// pushed to the right edge, which is how right-aligned table columns
// are produced.
func PadLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = Fit(s, w)
	if n := ansi.StringWidth(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}
