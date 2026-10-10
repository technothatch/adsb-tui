package render

import (
	"strings"
	"testing"
)

func TestBoxFraming(t *testing.T) {
	tests := []struct {
		name    string
		content []string
		w, h    int
		title   string
	}{
		{name: "standard with title", content: []string{"hello", "world"}, w: 20, h: 5, title: "range 60 nm"},
		{name: "no title", content: []string{"hello"}, w: 14, h: 3, title: ""},
		{name: "content longer than body", content: []string{"a", "b", "c", "d", "e"}, w: 12, h: 3, title: "aircraft"},
		{name: "content shorter pads blanks", content: []string{"only"}, w: 12, h: 4, title: ""},
		{name: "empty content", content: nil, w: 30, h: 3, title: "t"},
		{name: "single row box", content: nil, w: 10, h: 1, title: "t"},
		{name: "two row box", content: []string{"x"}, w: 10, h: 2, title: ""},
		{name: "degenerate narrow", content: []string{"x"}, w: 2, h: 3, title: "t"},
		{name: "one cell wide", content: []string{"x"}, w: 1, h: 3, title: "t"},
		{name: "long title falls back to plain", content: []string{"x"}, w: 16, h: 3, title: "a very long pane title"},
		{name: "title exactly filling the border", content: []string{"x"}, w: 12, h: 3, title: "ti"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := plainLines(Box(tt.content, tt.w, tt.h, tt.title, Dark()))
			if len(out) != tt.h {
				t.Fatalf("Box returned %d lines, want %d", len(out), tt.h)
			}
			for i, l := range out {
				if tt.w >= 2 && len(l) != tt.w {
					t.Errorf("line %d is %d cells, want %d: %q", i, len(l), tt.w, l)
				}
				if i == 0 && !strings.HasPrefix(l, "+") {
					t.Errorf("line %d does not start with the border: %q", i, l)
				}
				if i == tt.h-1 && !strings.HasPrefix(l, "+") {
					t.Errorf("last line %q is not the bottom border", l)
				}
			}
		})
	}
}

func TestBoxTitle(t *testing.T) {
	out := plainLines(Box([]string{"body"}, 20, 4, "range 60 nm", Dark()))
	top := out[0]
	if !strings.Contains(top, "range 60 nm") {
		t.Fatalf("top border %q does not contain the title", top)
	}
	if !strings.Contains(top, "+- ") {
		t.Errorf("top border %q should open with %+q", top, "+- ")
	}
	if top != "+- range 60 nm ----+" {
		t.Errorf("top border = %q, want %q", top, "+- range 60 nm ----+")
	}
	// Body is framed by pipes, bottom is plain dashes.
	if out[1] != "|body              |" {
		t.Errorf("body line = %q", out[1])
	}
	if out[3] != "+------------------+" {
		t.Errorf("bottom line = %q", out[3])
	}
}

func TestBoxMissingTitleKeepsPlainTop(t *testing.T) {
	out := plainLines(Box([]string{"x"}, 8, 4, "", Dark()))
	if out[0] != "+------+" {
		t.Fatalf("top = %q, want %q", out[0], "+------+")
	}
	// Empty body rows are spaces between pipes.
	if out[2] != "|      |" {
		t.Fatalf("blank body row = %q", out[2])
	}
}

func TestBoxClipsLongContentLines(t *testing.T) {
	out := plainLines(Box([]string{strings.Repeat("Z", 50)}, 10, 3, "", Dark()))
	body := out[1]
	if len(body) != 10 {
		t.Fatalf("body width = %d, want 10", len(body))
	}
	if !strings.Contains(body, "ZZZZZZ") || strings.Contains(body, strings.Repeat("Z", 9)) {
		t.Fatalf("body was not clipped correctly: %q", body)
	}
}

func TestBoxDegenerateSizes(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{name: "zero width", w: 0, h: 5},
		{name: "zero height", w: 5, h: 0},
		{name: "negative width", w: -4, h: 5},
		{name: "negative height", w: 5, h: -4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Box([]string{"x"}, tt.w, tt.h, "t", Dark()); got != nil {
				t.Fatalf("Box(%d, %d) = %v, want nil", tt.w, tt.h, got)
			}
		})
	}
}
