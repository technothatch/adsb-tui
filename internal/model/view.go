package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/technothatch/adsb-tui/internal/layout"
	"github.com/technothatch/adsb-tui/internal/render"
)

// View renders the whole frame: one status line on top, the content
// area in the middle and one footer line, exactly size.H lines of
// size.W cells each. Before the first WindowSizeMsg it renders nothing.
func (m Model) View() string {
	w, h := m.size.W, m.size.H
	if w <= 0 || h <= 0 {
		return ""
	}
	lines := make([]string, 0, h)
	lines = append(lines, m.headerLine())
	lines = append(lines, m.contentLines()...)
	lines = append(lines, m.footerLine())

	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = render.PadRight(l, w)
	}
	return strings.Join(lines, "\n")
}

// contentLines draws the content area: the key reference when help is
// open, otherwise the pane arrangement currently selected.
func (m Model) contentLines() []string {
	c := layout.Content(m.size)
	if c.W <= 0 || c.H <= 0 {
		return nil
	}
	th := m.theme
	switch {
	case m.help:
		return render.Box(render.Help(c.W, c.H-2, th), c.W, c.H, "keys", th)
	case m.pane == paneTable || !m.hasRecv:
		return render.Box(m.tableLines(c.W-2, c.H-2), c.W, c.H, m.tableTitle(), th)
	case m.pane == paneMap:
		return render.Box(m.rangeLines(c.W-2, c.H-2), c.W, c.H, m.rangeTitle(), th)
	}
	return m.splitLines(c)
}

// splitLines arranges the range display and the table: side by side on
// wide terminals (one blank gutter column between them) and stacked
// when the terminal is too narrow for two panes.
func (m Model) splitLines(c layout.Rect) []string {
	r, t := layout.Split(m.size)
	th := m.theme
	rangeBox := render.Box(m.rangeLines(r.W-2, r.H-2), r.W, r.H, m.rangeTitle(), th)
	tableBox := render.Box(m.tableLines(t.W-2, t.H-2), t.W, t.H, m.tableTitle(), th)

	if t.X <= r.X { // stacked (or the table alone)
		out := make([]string, 0, len(rangeBox)+len(tableBox))
		out = append(out, rangeBox...)
		out = append(out, tableBox...)
		return out
	}
	out := make([]string, 0, c.H)
	for i := 0; i < c.H; i++ {
		left, right := "", ""
		if i < len(rangeBox) {
			left = rangeBox[i]
		}
		if i < len(tableBox) {
			right = tableBox[i]
		}
		out = append(out, left+" "+right)
	}
	return out
}

// rangeLines draws the polar plot content for a pane of w x h cells.
func (m Model) rangeLines(w, h int) []string {
	targets := make([]render.Target, 0, len(m.entries))
	for i, e := range m.entries {
		targets = append(targets, e.Target(i == m.cursor))
	}
	opts := render.RangeOpts{
		W:        w,
		H:        h,
		RangeNm:  m.rangeNm,
		Targets:  targets,
		SweepDeg: m.sweep,
		Labels:   layout.LabelsFor(w, h),
		Theme:    m.theme,
	}
	if m.hasRecv && m.receiver.HasPosition() {
		opts.HasRecv = true
		opts.RecvLat = *m.receiver.Lat
		opts.RecvLon = *m.receiver.Lon
	}
	return render.Range(opts)
}

// tableLines draws the detail table for a pane of w x h cells,
// windowing the entries so only the visible rows are built.
func (m Model) tableLines(w, h int) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	cols := layout.TableColumns(w - 1) // one marker column in front
	rows := make([]render.TableRow, 0, h)
	if h > 2 {
		start, end := layout.Window(len(m.entries), h-2, m.cursor, m.offset)
		for i := start; i < end; i++ {
			rows = append(rows, m.rowFor(m.entries[i], i == m.cursor))
		}
	}
	return render.Table(render.TableOpts{
		W:       w,
		Height:  h,
		Columns: cols,
		Rows:    rows,
		Empty:   m.emptyHint(),
		Theme:   m.theme,
	})
}

// rowFor formats one aircraft as table cells.
func (m Model) rowFor(e Entry, selected bool) render.TableRow {
	cells := map[string]string{
		"hex":    e.Ac.Hex,
		"call":   e.Ac.Callsign(),
		"type":   e.Ac.TypeName(),
		"gs":     fmtFloat(e.Ac.GS),
		"track":  fmtFloat(e.Ac.Track),
		"squawk": e.Ac.Squawk,
	}
	alt, hasAlt := e.Ac.AltFeet()
	switch {
	case e.Ac.OnGround():
		cells["alt"] = "GND"
	case hasAlt:
		cells["alt"] = fmt.Sprintf("%.0f", alt)
	}
	if e.HasDist {
		cells["dst"] = fmt.Sprintf("%.1f", e.Dst)
		cells["dir"] = fmt.Sprintf("%03.0f", e.Dir)
	}
	return render.TableRow{
		Cells:     cells,
		Emergency: e.Ac.IsEmergency(),
		Selected:  selected,
	}
}

// fmtFloat renders an optional float as whole knots/degrees.
func fmtFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.0f", *v)
}

// emptyHint is the dim message shown in an empty table.
func (m Model) emptyHint() string {
	if !m.fetched {
		return "waiting for data..."
	}
	if m.filter != "" {
		return fmt.Sprintf("no aircraft match %q", m.filter)
	}
	return "no aircraft"
}

// rangeTitle and tableTitle label the pane frames.
func (m Model) rangeTitle() string {
	return fmt.Sprintf("range %.0f nm", m.rangeNm)
}

func (m Model) tableTitle() string {
	if m.filter != "" {
		return fmt.Sprintf("aircraft %d/%d", len(m.entries), len(m.aircraft))
	}
	return fmt.Sprintf("aircraft %d", len(m.aircraft))
}

// tableRect is the rectangle the detail table occupies in the current
// arrangement; a zero rectangle means the table is not visible (map
// pane or help overlay).
func (m Model) tableRect() layout.Rect {
	if m.help {
		return layout.Rect{}
	}
	if m.pane == paneMap && m.hasRecv {
		return layout.Rect{}
	}
	if m.pane == paneTable || !m.hasRecv {
		return layout.Content(m.size)
	}
	_, t := layout.Split(m.size)
	return t
}

// tableRowsHeight is how many data rows fit in the visible table:
// the pane height minus its frame (2), the header (1) and the
// separator (1).
func (m Model) tableRowsHeight() int {
	h := m.tableRect().H - 4
	if h < 0 {
		h = 0
	}
	return h
}

// headerLine is the one-line status bar.
func (m Model) headerLine() string {
	th := m.theme
	parts := []string{th.Style(render.AttrAccent).Render("adsb-tui")}

	count := fmt.Sprintf("%d ac", len(m.aircraft))
	if m.filter != "" {
		count = fmt.Sprintf("%d/%d ac", len(m.entries), len(m.aircraft))
	}
	parts = append(parts, th.Style(render.AttrDim).Render(count))
	parts = append(parts, "view "+m.pane.String())

	switch {
	case m.filtering:
		parts = append(parts, th.Style(render.AttrLabel).Render("/"+m.draft+"_"))
	case m.filter != "":
		parts = append(parts, th.Style(render.AttrLabel).Render("filter "+m.filter))
	}
	parts = append(parts, "sort "+m.sort.String())

	if n := m.emergencyCount(); n > 0 {
		parts = append(parts, th.Style(render.AttrWarn).Render(fmt.Sprintf("! emergency x%d", n)))
	}
	if m.paused {
		parts = append(parts, th.Style(render.AttrWarn).Render("PAUSED"))
	}
	if m.help {
		parts = append(parts, th.Style(render.AttrTitle).Render("help"))
	}
	if age := m.age(); age != "" {
		parts = append(parts, th.Style(render.AttrDim).Render(age))
	}
	return strings.Join(parts, "  ")
}

// footerLine is the one-line key hint, prefixed by a persistent
// warning or the last fetch error when there is one.
func (m Model) footerLine() string {
	th := m.theme
	hint := "q quit  t panes  / filter  h help  p pause  d theme  1/2/3 sort"
	if m.filtering {
		hint = "type to filter  enter apply  esc cancel"
	}
	line := th.Style(render.AttrDim).Render(hint)
	switch {
	case m.err != "":
		line = th.Style(render.AttrWarn).Render("err: "+m.err) + "  |  " + line
	case m.recvWarn != "":
		line = th.Style(render.AttrWarn).Render(m.recvWarn) + "  |  " + line
	}
	return line
}

// age is the "upd 2s" chip showing how stale the data is.
func (m Model) age() string {
	if m.updated.IsZero() || m.now.IsZero() {
		return ""
	}
	d := m.now.Sub(m.updated)
	if d < 0 {
		d = 0
	}
	return "upd " + d.Round(time.Second).String()
}
