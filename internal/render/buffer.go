package render

import "strings"

// cell is one character of a Buffer together with its semantic role.
type cell struct {
	r rune
	a Attr
}

// Buffer is a rectangular grid of rune cells. Drawing into it is
// bounds-checked, so out-of-range writes are silently dropped and every
// draw path can be tested without panicking.
type Buffer struct {
	// w and h are the grid dimensions in cells.
	w, h int
	// cells is the row-major grid, len == w*h.
	cells []cell
}

// NewBuffer allocates a w x h grid filled with spaces. Non-positive
// dimensions produce an empty buffer rather than an error: a pane too
// small to draw in simply stays blank.
func NewBuffer(w, h int) *Buffer {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	b := &Buffer{w: w, h: h, cells: make([]cell, w*h)}
	for i := range b.cells {
		b.cells[i] = cell{r: ' ', a: AttrNone}
	}
	return b
}

// Size returns the grid dimensions.
func (b *Buffer) Size() (w, h int) { return b.w, b.h }

// Set writes one cell. Writes outside the grid are dropped.
func (b *Buffer) Set(x, y int, r rune, a Attr) {
	if x < 0 || y < 0 || x >= b.w || y >= b.h {
		return
	}
	b.cells[y*b.w+x] = cell{r: r, a: a}
}

// At returns one cell; positions outside the grid report a space.
func (b *Buffer) At(x, y int) (rune, Attr) {
	if x < 0 || y < 0 || x >= b.w || y >= b.h {
		return ' ', AttrNone
	}
	c := b.cells[y*b.w+x]
	return c.r, c.a
}

// SetString writes s left to right starting at x, clipping at the right
// edge of the grid.
func (b *Buffer) SetString(x, y int, s string, a Attr) {
	for i, r := range []rune(s) {
		b.Set(x+i, y, r, a)
	}
}

// Fill writes a rectangle of the same cell.
func (b *Buffer) Fill(x, y, w, h int, r rune, a Attr) {
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			b.Set(x+dx, y+dy, r, a)
		}
	}
}

// Lines renders the grid as one string per row. Runs of equal
// attributes share a single style call, which keeps the ANSI escape
// count proportional to the number of colour changes rather than to the
// number of cells. Rows are exactly w cells wide.
func (b *Buffer) Lines(th Theme) []string {
	out := make([]string, 0, b.h)
	for y := 0; y < b.h; y++ {
		var sb strings.Builder
		var run []rune
		cur := AttrNone
		flush := func() {
			if len(run) == 0 {
				return
			}
			sb.WriteString(th.Style(cur).Render(string(run)))
			run = run[:0]
		}
		for x := 0; x < b.w; x++ {
			c := b.cells[y*b.w+x]
			if len(run) > 0 && c.a != cur {
				flush()
			}
			cur = c.a
			run = append(run, c.r)
		}
		flush()
		out = append(out, sb.String())
	}
	return out
}

// Plain renders the grid without any styling, one string per row. It is
// the introspection hook tests use to assert on cell contents.
func (b *Buffer) Plain() []string {
	out := make([]string, 0, b.h)
	for y := 0; y < b.h; y++ {
		var sb strings.Builder
		for x := 0; x < b.w; x++ {
			sb.WriteRune(b.cells[y*b.w+x].r)
		}
		out = append(out, sb.String())
	}
	return out
}
