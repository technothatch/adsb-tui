package render

import (
	"fmt"
	"math"

	"github.com/technothatch/adsb-tui/internal/layout"
)

// Target is one aircraft drawn on the range display. Position data
// comes from Lat/Lon when the receiver knows both ends; targets that
// only carry a decoder-computed Dst/Dir pair are plotted from those.
type Target struct {
	// Callsign is the trimmed flight identifier, e.g. "UAL123".
	Callsign string
	// Hex is the ICAO address, used for labels when no callsign exists.
	Hex string
	// Lat and Lon are the aircraft position in degrees.
	Lat, Lon float64
	// HasPos reports whether Lat/Lon are usable.
	HasPos bool
	// HasDist reports that DstNm/DirDeg are known (either computed
	// from both positions or supplied by the decoder).
	HasDist bool
	// DstNm and DirDeg are the decoder's distance/bearing from the
	// receiver, used when HasPos is false.
	DstNm, DirDeg float64
	// AltFeet is the barometric altitude in feet (0 on the ground or
	// unknown), shown in hundreds of feet in the blip label.
	AltFeet float64
	// OnGround marks the aircraft as taxiing or parked.
	OnGround bool
	// Emergency marks squawk 7500/7600/7700 or a decoder emergency.
	Emergency bool
	// Selected marks the aircraft currently highlighted in the table.
	Selected bool
}

// RangeOpts is everything the range display needs to draw itself.
type RangeOpts struct {
	// W and H are the content dimensions (inside any frame).
	W, H int
	// RangeNm is the ground distance mapped to the outer ring.
	RangeNm float64
	// RecvLat and RecvLon are the receiver position; HasRecv is false
	// when the decoder reported none (the caller then keeps the plot
	// off and shows the table alone).
	RecvLat, RecvLon float64
	HasRecv          bool
	// Targets are the aircraft to draw, in table order.
	Targets []Target
	// SweepDeg is the current radar sweep angle in degrees.
	SweepDeg float64
	// Labels selects the blip label degradation level.
	Labels layout.LabelMode
	// Theme is the active colour theme.
	Theme Theme
}

// Range draws the polar plot content — no frame — into a w x h grid
// and returns exactly h lines of w cells. Rings sit at 1/3, 2/3 and 1
// of the plotted range, cardinal letters mark the axes, and blips are
// '*', '@' (selected) or '!' (emergency).
func Range(o RangeOpts) []string {
	if o.W <= 0 || o.H <= 0 {
		return nil
	}
	buf := NewBuffer(o.W, o.H)
	th := o.Theme
	p := layout.NewPolar(o.W, o.H, o.RangeNm)
	cr := int(math.Round(p.CenterRow))
	cc := int(math.Round(p.CenterCol))

	// Crosshair: '-' along the centre row, '|' up the centre column.
	buf.Fill(0, cr, o.W, 1, '-', AttrRing)
	buf.Fill(cc, 0, 1, o.H, '|', AttrRing)

	// Range rings at 1/3, 2/3 and full scale.
	for _, frac := range []float64{1.0 / 3.0, 2.0 / 3.0, 1.0} {
		drawEllipse(buf, p, frac, '.', AttrRing)
	}

	// Cardinal letters over the crosshair ends.
	buf.Set(cc, 0, 'N', AttrDim)
	buf.Set(o.W-1, cr, 'E', AttrDim)
	buf.Set(cc, o.H-1, 'S', AttrDim)
	buf.Set(0, cr, 'W', AttrDim)

	// The sweep line, drawn after the rings so it stays visible.
	drawSweep(buf, p, o.SweepDeg)

	// Receiver centre.
	buf.Set(cc, cr, '+', AttrAccent)

	if o.Labels == layout.LabelsNone || o.RangeNm <= 0 {
		return buf.Lines(th)
	}

	type placed struct {
		row, col int
		label    string
	}
	var blips []placed
	blocked := map[[2]int]bool{}
	for _, t := range o.Targets {
		row, col, ok := posFor(p, t, o)
		if !ok {
			continue
		}
		if row < 0 || col < 0 || row >= o.H || col >= o.W {
			continue
		}
		mark, attr := '*', AttrBlip
		switch {
		case t.Selected:
			mark, attr = '@', AttrSel
		case t.Emergency:
			mark, attr = '!', AttrWarn
		}
		buf.Set(col, row, mark, attr)
		blocked[[2]int{col, row}] = true
		blips = append(blips, placed{row: row, col: col, label: labelFor(t)})
	}

	if o.Labels == layout.LabelsFull {
		for _, b := range blips {
			placeLabel(buf, blocked, b.row, b.col, b.label, o.W)
		}
	}
	return buf.Lines(th)
}

// posFor maps one target to a grid cell, preferring a fresh projection
// from both positions and falling back to the decoder's own
// distance/bearing pair.
func posFor(p layout.Polar, t Target, o RangeOpts) (row, col int, ok bool) {
	if t.HasPos && o.HasRecv {
		return p.Project(t.Lat, t.Lon, o.RecvLat, o.RecvLon)
	}
	if t.HasDist {
		return p.RangeBearing(t.DstNm, t.DirDeg)
	}
	return 0, 0, false
}

// labelFor builds the text next to a blip: callsign (or hex) plus the
// altitude in hundreds of feet, the way radar screens abbreviate FL.
func labelFor(t Target) string {
	name := t.Callsign
	if name == "" {
		name = t.Hex
	}
	if name == "" {
		return ""
	}
	if t.OnGround {
		return name + " GND"
	}
	if t.AltFeet > 0 {
		return fmt.Sprintf("%s %d", name, int(t.AltFeet)/100)
	}
	return name
}

// placeLabel writes label to the right of a blip, falling back to its
// left when the right side does not fit. Cells already holding a blip
// are never overwritten, so a label can wrap around a neighbour
// without erasing it.
func placeLabel(buf *Buffer, blocked map[[2]int]bool, row, col int, label string, w int) {
	if label == "" {
		return
	}
	runes := []rune(label)
	start := col + 2
	if start+len(runes) > w {
		start = col - 1 - len(runes)
	}
	if start < 0 {
		return
	}
	for i, r := range runes {
		x := start + i
		if x < 0 || x >= w {
			continue
		}
		if blocked[[2]int{x, row}] {
			continue
		}
		buf.Set(x, row, r, AttrLabel)
		blocked[[2]int{x, row}] = true
	}
}

// drawEllipse stamps one range ring: the ellipse traced by a constant
// ground distance on a grid whose cells are twice as tall as wide.
func drawEllipse(b *Buffer, p layout.Polar, frac float64, r rune, a Attr) {
	steps := int(2*math.Pi*math.Max(p.ColRadius, p.RowRadius)) + 8
	if steps < 16 {
		steps = 16
	}
	for i := 0; i < steps; i++ {
		t := float64(i) / float64(steps) * 2 * math.Pi
		row := int(math.Round(p.CenterRow - math.Cos(t)*frac*p.RowRadius))
		col := int(math.Round(p.CenterCol + math.Sin(t)*frac*p.ColRadius))
		b.Set(col, row, r, a)
	}
}

// drawSweep stamps the radar sweep as a radial line of ':' cells at
// the current angle.
func drawSweep(b *Buffer, p layout.Polar, deg float64) {
	steps := int(math.Max(p.ColRadius, p.RowRadius))
	if steps < 1 {
		return
	}
	rad := deg * math.Pi / 180
	for i := 0; i <= steps; i++ {
		f := float64(i) / float64(steps)
		row := int(math.Round(p.CenterRow - math.Cos(rad)*f*p.RowRadius))
		col := int(math.Round(p.CenterCol + math.Sin(rad)*f*p.ColRadius))
		b.Set(col, row, ':', AttrSweep)
	}
}
