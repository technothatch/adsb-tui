package render

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFitPad(t *testing.T) {
	tests := []struct {
		name string
		op   string
		fn   func(string, int) string
		in   string
		w    int
		want string
	}{
		{name: "fit keeps short text", op: "Fit", fn: Fit, in: "hi", w: 10, want: "hi"},
		{name: "fit clips long text", op: "Fit", fn: Fit, in: "hello", w: 3, want: "hel"},
		{name: "fit at exact width", op: "Fit", fn: Fit, in: "hello", w: 5, want: "hello"},
		{name: "fit non-positive width", op: "Fit", fn: Fit, in: "hello", w: 0, want: ""},
		{name: "pad right grows", op: "PadRight", fn: PadRight, in: "ab", w: 4, want: "ab  "},
		{name: "pad right already wide", op: "PadRight", fn: PadRight, in: "abcd", w: 4, want: "abcd"},
		{name: "pad right clips", op: "PadRight", fn: PadRight, in: "abcdef", w: 3, want: "abc"},
		{name: "pad right zero width", op: "PadRight", fn: PadRight, in: "abc", w: 0, want: ""},
		{name: "pad left grows", op: "PadLeft", fn: PadLeft, in: "ab", w: 4, want: "  ab"},
		{name: "pad left clips to width", op: "PadLeft", fn: PadLeft, in: "abcdef", w: 2, want: "ab"},
		{name: "pad left non-positive width", op: "PadLeft", fn: PadLeft, in: "abc", w: -1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.in, tt.w); got != tt.want {
				t.Fatalf("%s(%q, %d) = %q, want %q", tt.op, tt.in, tt.w, got, tt.want)
			}
		})
	}
}

func TestPadKeepsANSIWidth(t *testing.T) {
	styled := "\x1b[31mred\x1b[0m"
	if got := ansi.StringWidth(PadRight(styled, 8)); got != 8 {
		t.Fatalf("PadRight(styled, 8) width = %d, want 8", got)
	}
	if got := ansi.StringWidth(PadLeft(styled, 6)); got != 6 {
		t.Fatalf("PadLeft(styled, 6) width = %d, want 6", got)
	}
	// Escape sequences must survive truncation so styling stays intact.
	out := Fit("\x1b[1mbold text here\x1b[0m", 4)
	if got := ansi.StringWidth(out); got != 4 {
		t.Fatalf("Fit(styled, 4) width = %d, want 4", got)
	}
}
