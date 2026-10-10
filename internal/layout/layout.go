// Package layout computes terminal geometry: pane rectangles, the polar
// projection used by the range display, the visible window of the detail
// table and the set of table columns that fit a given width. Everything
// here is pure math with no I/O, so it is tested table-driven.
package layout

import "math"

// Rect is a rectangle of character cells, with the origin at the top left.
type Rect struct {
	// X and Y are the top-left cell of the rectangle.
	X, Y int
	// W and H are the width and height in cells.
	W, H int
}

// Size is a terminal size in cells.
type Size struct {
	// W is the number of columns.
	W int
	// H is the number of rows.
	H int
}

// headerRows and footerRows are reserved by every view: one status header
// row on top, one key-hint/warning row at the bottom.
const (
	headerRows = 1
	footerRows = 1
)

// Content returns the rectangle left between the header and the footer.
// A zero rectangle is returned when the terminal is too small to hold
// even the two status rows.
func Content(size Size) Rect {
	if size.W <= 0 || size.H <= 0 {
		return Rect{}
	}
	h := size.H - headerRows - footerRows
	if h <= 0 {
		return Rect{}
	}
	return Rect{X: 0, Y: headerRows, W: size.W, H: h}
}

// Split divides the content area into a range pane and a table pane.
// Wide terminals get the panes side by side with a one-column gutter,
// narrower ones get the range display stacked above the table, and very
// small ones get the table alone (a zero range rect). The returned
// rectangles are always inside Content(size) and never overlap.
func Split(size Size) (rangePane, tablePane Rect) {
	c := Content(size)
	if c.W <= 0 || c.H <= 0 {
		return Rect{}, Rect{}
	}
	if c.W >= 70 {
		rangeW := c.W * 45 / 100
		if rangeW < 32 {
			rangeW = 32
		}
		if rangeW > 60 {
			rangeW = 60
		}
		if c.W-rangeW-1 < 28 {
			rangeW = c.W - 29
		}
		if rangeW < 20 {
			return Rect{}, c
		}
		return Rect{X: 0, Y: c.Y, W: rangeW, H: c.H},
			Rect{X: rangeW + 1, Y: c.Y, W: c.W - rangeW - 1, H: c.H}
	}
	if c.W >= 28 && c.H >= 8 {
		rangeH := c.H * 55 / 100
		if rangeH < 4 {
			rangeH = 4
		}
		if c.H-rangeH < 4 {
			rangeH = c.H - 4
		}
		return Rect{X: 0, Y: c.Y, W: c.W, H: rangeH},
			Rect{X: 0, Y: c.Y + rangeH, W: c.W, H: c.H - rangeH}
	}
	return Rect{}, c
}

// Window returns the half-open index range [start, end) of the n items
// that fit into a viewport of the given height while keeping cursor
// visible. The previous start position is reused when it still shows the
// cursor, so the list does not jump around while data refreshes.
func Window(n, height, cursor, start int) (int, int) {
	if n <= 0 || height <= 0 {
		return 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= n {
		cursor = n - 1
	}
	if start < 0 {
		start = 0
	}
	if start > n-height {
		start = n - height
	}
	if start < 0 {
		start = 0
	}
	if cursor < start {
		start = cursor
	}
	if cursor >= start+height {
		start = cursor - height + 1
	}
	if start > n-height {
		start = n - height
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > n {
		end = n
	}
	return start, end
}

// LabelMode controls how much text the range display draws next to each
// aircraft blip. It degrades as the pane shrinks so labels never crowd
// the plot out.
type LabelMode int

// The three degradation levels of the range display.
const (
	// LabelsNone draws no aircraft at all (pane far too small).
	LabelsNone LabelMode = iota
	// LabelsMarker draws a one-character blip per aircraft only.
	LabelsMarker
	// LabelsFull draws the blip plus a callsign/altitude label.
	LabelsFull
)

// LabelsFor picks the label mode for a range pane of the given size.
func LabelsFor(w, h int) LabelMode {
	switch {
	case w >= 30 && h >= 10:
		return LabelsFull
	case w >= 12 && h >= 4:
		return LabelsMarker
	default:
		return LabelsNone
	}
}

// Column describes one column of the detail table. Width is the fixed
// character width of the column; Drop ranks which column is sacrificed
// first when the pane narrows (higher drops first, lower survives
// longest).
type Column struct {
	// Key identifies the column when formatting a row.
	Key string
	// Title is the column heading.
	Title string
	// Width is the fixed width of the column in cells.
	Width int
	// Right aligns the cell content inside the column.
	Right bool
	// Drop is the drop priority: higher values are dropped first.
	Drop int
}

// columns lists every known column in display order. TableColumns picks
// a fitting subset of it.
var columns = []Column{
	{Key: "hex", Title: "HEX", Width: 6, Drop: 1},
	{Key: "call", Title: "CALLSIGN", Width: 8, Drop: 1},
	{Key: "type", Title: "TYPE", Width: 4, Drop: 8},
	{Key: "alt", Title: "ALT", Width: 6, Right: true, Drop: 3},
	{Key: "gs", Title: "GS", Width: 5, Right: true, Drop: 6},
	{Key: "track", Title: "TRK", Width: 5, Right: true, Drop: 7},
	{Key: "squawk", Title: "SQ", Width: 5, Drop: 4},
	{Key: "dst", Title: "DST", Width: 6, Right: true, Drop: 5},
	{Key: "dir", Title: "BRG", Width: 4, Right: true, Drop: 9},
}

// Columns returns every known column in display order. Callers that need
// only a fitting subset should use TableColumns.
func Columns() []Column {
	out := make([]Column, len(columns))
	copy(out, columns)
	return out
}

// TableColumns returns the subset of Columns that fits into width cells
// (including one-cell gutters between columns), dropping the least
// important columns first. At least one column is returned for any
// positive width; a non-positive width returns no columns.
func TableColumns(width int) []Column {
	if width <= 0 {
		return nil
	}
	cols := Columns()
	for len(cols) > 1 && columnsWidth(cols) > width {
		victim := 0
		for i, c := range cols {
			if c.Drop > cols[victim].Drop || (c.Drop == cols[victim].Drop && i > victim) {
				victim = i
			}
		}
		cols = append(cols[:victim], cols[victim+1:]...)
	}
	return cols
}

// columnsWidth is the rendered width of cols including gutters.
func columnsWidth(cols []Column) int {
	total := 0
	for i, c := range cols {
		if i > 0 {
			total++
		}
		total += c.Width
	}
	return total
}

// Polar projects geographic points onto a character grid so that a
// circle on the ground appears round on screen. Terminal cells are about
// twice as tall as they are wide, so the vertical radius of the plot is
// half the horizontal one.
type Polar struct {
	// CenterRow and CenterCol are the grid coordinates of the receiver.
	CenterRow float64
	CenterCol float64
	// RowRadius and ColRadius are the plot radii in cells.
	RowRadius float64
	ColRadius float64
	// RangeNm is the ground distance mapped to the outer ring.
	RangeNm float64
}

// NewPolar builds the projection for a grid of the given size and range
// in nautical miles. The circle is sized to fit inside the grid: the
// horizontal radius is preferred and the vertical radius is halved to
// compensate for the cell aspect ratio, then clamped to the grid height.
func NewPolar(cellsW, cellsH int, rangeNm float64) Polar {
	p := Polar{
		CenterRow: float64(cellsH-1) / 2,
		CenterCol: float64(cellsW-1) / 2,
		RangeNm:   rangeNm,
	}
	colRadius := (float64(cellsW) - 1) / 2
	rowRadius := colRadius / 2
	if maxRows := (float64(cellsH) - 1) / 2; rowRadius > maxRows {
		rowRadius = maxRows
		colRadius = rowRadius * 2
	}
	if colRadius < 0 {
		colRadius = 0
	}
	if rowRadius < 0 {
		rowRadius = 0
	}
	p.ColRadius = colRadius
	p.RowRadius = rowRadius
	return p
}

// Project maps an aircraft position onto the grid, relative to the
// receiver position. ok is false when either position is invalid or the
// aircraft lies outside the plotted range.
func (p Polar) Project(lat, lon, recvLat, recvLon float64) (row, col int, ok bool) {
	if p.RangeNm <= 0 {
		return 0, 0, false
	}
	if !validLatLon(lat, lon) || !validLatLon(recvLat, recvLon) {
		return 0, 0, false
	}
	dst := HaversineNm(recvLat, recvLon, lat, lon)
	return p.RangeBearing(dst, BearingDeg(recvLat, recvLon, lat, lon))
}

// RangeBearing maps a distance and bearing from the receiver onto the
// grid. ok is false when the distance is outside the plotted range.
func (p Polar) RangeBearing(dstNm, bearingDeg float64) (row, col int, ok bool) {
	if p.RangeNm <= 0 || dstNm < 0 || dstNm > p.RangeNm || math.IsNaN(dstNm) {
		return 0, 0, false
	}
	frac := dstNm / p.RangeNm
	rad := bearingDeg * math.Pi / 180
	row = int(math.Round(p.CenterRow - math.Cos(rad)*frac*p.RowRadius))
	col = int(math.Round(p.CenterCol + math.Sin(rad)*frac*p.ColRadius))
	return row, col, true
}

// validLatLon reports whether a position is inside the WGS84 degree
// ranges. NaN comparisons are false, so NaN fails both bounds.
func validLatLon(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// earthRadiusNm is the mean Earth radius in nautical miles.
const earthRadiusNm = 3440.065

// HaversineNm returns the great-circle distance between two positions in
// nautical miles.
func HaversineNm(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusNm * c
}

// BearingDeg returns the initial bearing from position 1 to position 2
// in degrees, clockwise from true north, in [0, 360).
func BearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	const rad = math.Pi / 180
	phi1 := lat1 * rad
	phi2 := lat2 * rad
	dLon := (lon2 - lon1) * rad
	x := math.Sin(dLon) * math.Cos(phi2)
	y := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(dLon)
	b := math.Atan2(x, y) * 180 / math.Pi
	if b < 0 {
		b += 360
	}
	return b
}
