package render

import (
	"regexp"
	"strings"
	"testing"
)

// strip removes SGR sequences so assertions see plain text.
var strip = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString

// plainLines strips every rendered line.
func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strip(l, "")
	}
	return out
}

func TestBufferBasics(t *testing.T) {
	t.Run("dimensions", func(t *testing.T) {
		b := NewBuffer(4, 3)
		w, h := b.Size()
		if w != 4 || h != 3 {
			t.Fatalf("size = (%d, %d), want (4, 3)", w, h)
		}
		lines := b.Plain()
		if len(lines) != 3 {
			t.Fatalf("Plain() = %d lines, want 3", len(lines))
		}
		for i, l := range lines {
			if l != "    " {
				t.Fatalf("line %d = %q, want blank", i, l)
			}
		}
	})
	t.Run("set and read back", func(t *testing.T) {
		b := NewBuffer(4, 3)
		b.Set(1, 1, 'x', AttrBlip)
		r, a := b.At(1, 1)
		if r != 'x' || a != AttrBlip {
			t.Fatalf("At(1,1) = (%q, %v), want (x, blip)", r, a)
		}
	})
	t.Run("writes outside the grid are dropped", func(t *testing.T) {
		b := NewBuffer(4, 3)
		b.Set(-1, 0, 'a', AttrBlip)
		b.Set(0, -1, 'b', AttrBlip)
		b.Set(4, 0, 'c', AttrBlip)
		b.Set(0, 3, 'd', AttrBlip)
		for _, l := range b.Plain() {
			if strings.ContainsAny(l, "abcd") {
				t.Fatalf("out-of-bounds write landed: %q", l)
			}
		}
	})
	t.Run("reads outside the grid report a space", func(t *testing.T) {
		b := NewBuffer(4, 3)
		for _, tc := range [][2]int{{-1, 0}, {0, -1}, {4, 0}, {0, 3}} {
			r, a := b.At(tc[0], tc[1])
			if r != ' ' || a != AttrNone {
				t.Errorf("At(%d,%d) = (%q, %v), want blank", tc[0], tc[1], r, a)
			}
		}
	})
	t.Run("SetString clips at the right edge", func(t *testing.T) {
		b := NewBuffer(4, 3)
		b.SetString(2, 0, "abcd", AttrNone)
		b.SetString(-1, 1, "xy", AttrNone)
		lines := b.Plain()
		if lines[0] != "  ab" {
			t.Fatalf("row 0 = %q, want %q", lines[0], "  ab")
		}
		if lines[1] != "y   " {
			t.Fatalf("row 1 = %q, want %q", lines[1], "y   ")
		}
	})
	t.Run("Fill clips to the grid", func(t *testing.T) {
		b := NewBuffer(4, 3)
		b.Fill(1, 1, 10, 10, 'f', AttrNone)
		if r, _ := b.At(3, 2); r != 'f' {
			t.Errorf("At(3,2) = %q, want f", r)
		}
		if r, _ := b.At(0, 0); r != ' ' {
			t.Errorf("Fill escaped the grid: At(0,0) = %q", r)
		}
	})
	t.Run("Lines render the grid at exact width", func(t *testing.T) {
		b := NewBuffer(4, 3)
		b.SetString(0, 1, "hi", AttrBlip)
		lines := plainLines(b.Lines(Dark()))
		if len(lines) != 3 {
			t.Fatalf("Lines() = %d lines, want 3", len(lines))
		}
		for i, l := range lines {
			if len(l) != 4 {
				t.Fatalf("line %d = %q, want 4 cells", i, l)
			}
		}
		if lines[1] != "hi  " {
			t.Fatalf("row 1 = %q, want %q", lines[1], "hi  ")
		}
	})
	t.Run("non-positive sizes stay empty", func(t *testing.T) {
		b := NewBuffer(-2, -5)
		w, h := b.Size()
		if w != 0 || h != 0 {
			t.Fatalf("size = (%d, %d), want (0, 0)", w, h)
		}
		b.Set(0, 0, 'x', AttrNone) // must not panic
		if got := b.Lines(Dark()); len(got) != 0 {
			t.Fatalf("Lines() = %d lines, want 0", len(got))
		}
		if got := b.Plain(); len(got) != 0 {
			t.Fatalf("Plain() = %d lines, want 0", len(got))
		}
	})
}

func TestBufferAttributesSurviveRendering(t *testing.T) {
	b := NewBuffer(3, 1)
	b.SetString(0, 0, "ab", AttrWarn)
	b.Set(2, 0, 'c', AttrNone)
	lines := b.Lines(Dark())
	if len(lines) != 1 {
		t.Fatalf("Lines() = %d lines, want 1", len(lines))
	}
	if plain := strip(lines[0], ""); plain != "abc" {
		t.Fatalf("rendered %q, want %q", plain, "abc")
	}
}
