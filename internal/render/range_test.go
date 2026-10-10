package render

import (
	"strings"
	"testing"

	"github.com/technothatch/adsb-tui/internal/layout"
)

// rangeFixture is a 41x21 pane (60 nm scale) with a selected aircraft
// north of the receiver, an emergency aircraft east of it, and a
// ground target plotted purely from a decoder distance/bearing pair.
func rangeFixture() RangeOpts {
	return RangeOpts{
		W: 41, H: 21, RangeNm: 60,
		HasRecv: true, RecvLat: 48.2, RecvLon: 11.5,
		Targets: []Target{
			{Callsign: "UAL123", Hex: "aaaaaa", Lat: 48.3, Lon: 11.5,
				HasPos: true, HasDist: true, DstNm: 6, DirDeg: 0,
				AltFeet: 35000, Selected: true},
			{Callsign: "DLH441", Hex: "bbbbbb", Lat: 48.2, Lon: 11.6,
				HasPos: true, HasDist: true, DstNm: 4, DirDeg: 90,
				AltFeet: 12000, Emergency: true},
			{Hex: "cccccc", HasDist: true, DstNm: 40, DirDeg: 270, OnGround: true},
		},
		SweepDeg: 45,
		Labels:   layout.LabelsFull,
		Theme:    Dark(),
	}
}

func TestRangeFrame(t *testing.T) {
	o := rangeFixture()
	out := plainLines(Range(o))
	if len(out) != o.H {
		t.Fatalf("Range returned %d lines, want %d", len(out), o.H)
	}
	for i, l := range out {
		if len(l) != o.W {
			t.Errorf("line %d is %d cells, want %d", i, len(l), o.W)
		}
	}
}

func TestRangeCardinalsCentreAndRings(t *testing.T) {
	o := rangeFixture()
	out := plainLines(Range(o))
	if out[0][20] != 'N' {
		t.Errorf("north pole = %q, want N", out[0][20])
	}
	if out[o.H-1][20] != 'S' {
		t.Errorf("south pole = %q, want S", out[o.H-1][20])
	}
	if out[10][0] != 'W' {
		t.Errorf("west pole = %q, want W", out[10][0])
	}
	if out[10][o.W-1] != 'E' {
		t.Errorf("east pole = %q, want E", out[10][o.W-1])
	}
	if out[10][20] != '+' {
		t.Errorf("centre = %q, want the + receiver mark", out[10][20])
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, ".") {
		t.Error("no range ring dots drawn")
	}
	if !strings.Contains(joined, ":") {
		t.Error("no sweep line drawn")
	}
	if !strings.Contains(joined, "-") || !strings.Contains(joined, "|") {
		t.Error("crosshair missing")
	}
}

func TestRangeBlipsAndLabels(t *testing.T) {
	o := rangeFixture()
	out := plainLines(Range(o))

	// Selected aircraft: 6 nm north of the receiver, '@' on the grid.
	if got := out[9][20]; got != '@' {
		t.Errorf("selected blip at (20,9) = %q, want @", got)
	}
	// Emergency aircraft: ~4 nm east, '!' on the grid.
	if got := out[10][21]; got != '!' {
		t.Errorf("emergency blip at (21,10) = %q, want !", got)
	}
	// Ground target from dst/dir only: 40 nm west, '*' on the grid.
	if got := out[10][7]; got != '*' {
		t.Errorf("ground blip at (7,10) = %q, want *", got)
	}

	joined := strings.Join(out, "\n")
	for _, want := range []string{"UAL123 350", "DLH441 120", "cccccc GND"} {
		if !strings.Contains(joined, want) {
			t.Errorf("label %q missing from the plot", want)
		}
	}
	// The label sits beside the blip, on the same row.
	if row := out[9]; !strings.Contains(row[22:32], "UAL123 350") {
		t.Errorf("row 9 = %q, want the label at columns 22..31", row)
	}
}

func TestRangeLabelModes(t *testing.T) {
	tests := []struct {
		name   string
		labels layout.LabelMode
		plot   bool
		text   bool
	}{
		{name: "full shows blips and text", labels: layout.LabelsFull, plot: true, text: true},
		{name: "marker shows blips only", labels: layout.LabelsMarker, plot: true, text: false},
		{name: "none draws nothing", labels: layout.LabelsNone, plot: false, text: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := rangeFixture()
			o.Labels = tt.labels
			joined := strings.Join(plainLines(Range(o)), "\n")
			if tt.plot {
				for _, mark := range []string{"@", "!", "*"} {
					if !strings.Contains(joined, mark) {
						t.Errorf("missing blip %q", mark)
					}
				}
			} else {
				for _, mark := range []string{"@", "!", "*"} {
					if strings.Contains(joined, mark) {
						t.Errorf("unexpected blip %q", mark)
					}
				}
			}
			if tt.text {
				if !strings.Contains(joined, "UAL123") {
					t.Error("missing callsign label")
				}
			} else {
				if strings.Contains(joined, "UAL123") || strings.Contains(joined, "GND") {
					t.Error("labels drawn in marker/none mode")
				}
			}
		})
	}
}

func TestRangeSweepAngleChangesOutput(t *testing.T) {
	a := rangeFixture()
	a.SweepDeg = 45
	b := rangeFixture()
	b.SweepDeg = 225
	ja := strings.Join(plainLines(Range(a)), "\n")
	jb := strings.Join(plainLines(Range(b)), "\n")
	if ja == jb {
		t.Fatal("sweep angle has no visible effect")
	}
}

func TestRangeProjectionFallbacks(t *testing.T) {
	t.Run("latlon needs the receiver", func(t *testing.T) {
		o := rangeFixture()
		o.HasRecv = false
		o.Targets = []Target{
			{Callsign: "POSNLY", Lat: 48.3, Lon: 11.5, HasPos: true},
		}
		joined := strings.Join(plainLines(Range(o)), "\n")
		if strings.Contains(joined, "POSNLY") {
			t.Error("target plotted without a receiver position")
		}
	})
	t.Run("distance and bearing plot without a receiver", func(t *testing.T) {
		o := rangeFixture()
		o.HasRecv = false
		o.Targets = []Target{
			{Callsign: "DIST9", Lat: 48.3, Lon: 11.5, HasPos: true,
				HasDist: true, DstNm: 6, DirDeg: 180},
		}
		joined := strings.Join(plainLines(Range(o)), "\n")
		if !strings.Contains(joined, "DIST9") {
			t.Errorf("dst/dir fallback did not plot: %q", joined)
		}
		// 6 nm due south: one ring radius below the centre.
		if got := plainLines(Range(o))[11][20]; got != '*' {
			t.Errorf("blip at (20,11) = %q, want *", got)
		}
	})
	t.Run("targets beyond the plotted range are skipped", func(t *testing.T) {
		o := rangeFixture()
		o.Targets = []Target{
			{Callsign: "FAR11", HasDist: true, DstNm: 90, DirDeg: 45},
		}
		joined := strings.Join(plainLines(Range(o)), "\n")
		if strings.Contains(joined, "FAR11") {
			t.Error("out-of-range target was drawn")
		}
	})
	t.Run("target with no usable position is skipped", func(t *testing.T) {
		o := rangeFixture()
		o.Targets = []Target{{Callsign: "GHOST"}}
		joined := strings.Join(plainLines(Range(o)), "\n")
		if strings.Contains(joined, "GHOST") {
			t.Error("positionless target was drawn")
		}
	})
}

func TestRangeDegenerateSizes(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{name: "zero width", w: 0, h: 10},
		{name: "zero height", w: 10, h: 0},
		{name: "negative width", w: -5, h: 10},
		{name: "negative height", w: 10, h: -5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := rangeFixture()
			o.W, o.H = tt.w, tt.h
			if got := Range(o); got != nil {
				t.Fatalf("Range(%d, %d) = %v, want nil", tt.w, tt.h, got)
			}
		})
	}
}

func TestRangeZeroRangeScale(t *testing.T) {
	o := rangeFixture()
	o.RangeNm = 0
	out := plainLines(Range(o))
	if len(out) != o.H {
		t.Fatalf("Range returned %d lines, want %d", len(out), o.H)
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "UAL123") || strings.Contains(joined, "@") {
		t.Error("aircraft drawn at zero range scale")
	}
}

func TestRangeOneCellGrid(t *testing.T) {
	o := rangeFixture()
	o.W, o.H = 1, 1
	o.Labels = layout.LabelsNone
	out := plainLines(Range(o))
	if len(out) != 1 || len(out[0]) != 1 {
		t.Fatalf("Range(1x1) = %v, want one single cell", out)
	}
	if out[0][0] != '+' {
		t.Errorf("cell = %q, want the + receiver mark", out[0][0])
	}
}
