package render

import "github.com/technothatch/adsb-tui/internal/layout"

// TableRow is one row of the detail table. Cells maps column keys
// (see layout.Column.Key) to the plain text to display; missing keys
// render as blanks. Cell text must be plain — styling comes from the
// theme via the row flags.
type TableRow struct {
	// Cells holds the plain text per column key.
	Cells map[string]string
	// Emergency colours the row for squawk 7500/7600/7700.
	Emergency bool
	// Selected marks the cursor row.
	Selected bool
}

// TableOpts is everything the detail table needs to draw itself.
type TableOpts struct {
	// W is the total width in cells; column text is clipped to it.
	W int
	// Height is the exact number of lines to return.
	Height int
	// Columns are the columns that fit width W (see
	// layout.TableColumns), in display order.
	Columns []layout.Column
	// Rows are the visible rows; rows beyond Height are dropped.
	Rows []TableRow
	// Empty is an optional dim hint shown when Rows is empty.
	Empty string
	// Theme is the active colour theme.
	Theme Theme
}

// Table renders the header, a separator and the visible rows into
// exactly Height lines of W cells. The first cell of every line is a
// marker column: '>' for the cursor row, '!' for an emergency row, a
// space otherwise, so selection stays readable even without colour.
func Table(o TableOpts) []string {
	if o.W <= 0 || o.Height <= 0 {
		return nil
	}
	buf := NewBuffer(o.W, o.Height)

	// Header.
	x := 1 // marker column
	for _, c := range o.Columns {
		if x > o.W {
			break
		}
		writeCell(buf, x, 0, c, c.Title, AttrHeader)
		x += c.Width + 1
	}
	// Separator.
	if o.Height > 1 {
		buf.Fill(0, 1, o.W, 1, '-', AttrDim)
	}

	y := 2
	for _, r := range o.Rows {
		if y >= o.Height {
			break
		}
		drawRow(buf, y, o.W, o.Columns, r)
		y++
	}
	if len(o.Rows) == 0 && o.Empty != "" && y < o.Height {
		buf.SetString(1, y, o.Empty, AttrDim)
	}
	return buf.Lines(o.Theme)
}

// drawRow paints one data row: an optional background band for the
// selection or emergency state, the marker cell and the column cells.
func drawRow(buf *Buffer, y, w int, cols []layout.Column, r TableRow) {
	attr := AttrNone
	switch {
	case r.Selected:
		attr = AttrSel
	case r.Emergency:
		attr = AttrWarn
	}
	if attr != AttrNone {
		buf.Fill(0, y, w, 1, ' ', attr)
	}
	mark := ' '
	switch {
	case r.Selected && r.Emergency:
		mark = '!'
	case r.Selected:
		mark = '>'
	case r.Emergency:
		mark = '!'
	}
	buf.Set(0, y, mark, attr)

	x := 1
	for _, c := range cols {
		if x > w {
			break
		}
		writeCell(buf, x, y, c, r.Cells[c.Key], attr)
		x += c.Width + 1
	}
}

// writeCell writes text into one fixed-width column, padded on the
// side the column declares. Text is plain (no escape sequences): the
// attribute carries the styling.
func writeCell(buf *Buffer, x, y int, c layout.Column, text string, a Attr) {
	var s string
	if c.Right {
		s = PadLeft(text, c.Width)
	} else {
		s = PadRight(text, c.Width)
	}
	buf.SetString(x, y, s, a)
}
