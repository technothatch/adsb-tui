package layout

import (
	"math"
	"testing"
)

func TestContent(t *testing.T) {
	tests := []struct {
		name string
		size Size
		want Rect
	}{
		{name: "typical", size: Size{W: 80, H: 24}, want: Rect{X: 0, Y: 1, W: 80, H: 22}},
		{name: "wide", size: Size{W: 200, H: 50}, want: Rect{X: 0, Y: 1, W: 200, H: 48}},
		{name: "only header and footer", size: Size{W: 10, H: 2}, want: Rect{}},
		{name: "one row", size: Size{W: 10, H: 1}, want: Rect{}},
		{name: "zero width", size: Size{W: 0, H: 24}, want: Rect{}},
		{name: "zero height", size: Size{W: 80, H: 0}, want: Rect{}},
		{name: "negative", size: Size{W: -5, H: -5}, want: Rect{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Content(tt.size)
			if got != tt.want {
				t.Fatalf("Content(%+v) = %+v, want %+v", tt.size, got, tt.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name       string
		size       Size
		wantSide   bool // panes side by side with a gutter
		wantStack  bool // range above table, full width
		wantTableW int  // expected table pane width
		wantRangeW int  // expected range pane width (0 when hidden)
	}{
		{
			name: "80x24 side by side", size: Size{W: 80, H: 24},
			wantSide: true, wantRangeW: 36, wantTableW: 43,
		},
		{
			name: "200x50 caps range pane at 60", size: Size{W: 200, H: 50},
			wantSide: true, wantRangeW: 60, wantTableW: 139,
		},
		{
			name: "69 wide stacks", size: Size{W: 69, H: 30},
			wantStack: true, wantRangeW: 69, wantTableW: 69,
		},
		{
			name: "narrow short falls back to table", size: Size{W: 40, H: 6},
			wantTableW: 40, wantRangeW: 0,
		},
		{
			name: "tiny terminal", size: Size{W: 10, H: 3},
			wantTableW: 10, wantRangeW: 0,
		},
		{
			name: "zero size", size: Size{W: 0, H: 0},
			wantTableW: 0, wantRangeW: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, tb := Split(tt.size)
			if tt.wantRangeW == 0 && tt.wantTableW == 0 {
				if r != (Rect{}) || tb != (Rect{}) {
					t.Fatalf("Split(%+v) = %+v, %+v; want zero rects", tt.size, r, tb)
				}
				return
			}
			if r.W != tt.wantRangeW {
				t.Errorf("range width = %d, want %d", r.W, tt.wantRangeW)
			}
			if tb.W != tt.wantTableW {
				t.Errorf("table width = %d, want %d", tb.W, tt.wantTableW)
			}
			side := tt.size.W >= 70 && tt.wantRangeW > 0
			if side != tt.wantSide {
				t.Errorf("side by side = %v, want %v", side, tt.wantSide)
			}
			c := Content(tt.size)
			if tt.wantSide {
				if r.H != c.H || tb.H != c.H {
					t.Errorf("pane heights r=%d t=%d, want %d", r.H, tb.H, c.H)
				}
			}
			if tb.X+tb.W > c.W {
				t.Errorf("table pane overflows content: %+v", tb)
			}
		})
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		name          string
		n, height     int
		cursor, start int
		wantStart     int
		wantEnd       int
	}{
		{name: "fits entirely", n: 3, height: 10, cursor: 0, start: 0, wantStart: 0, wantEnd: 3},
		{name: "cursor at top", n: 10, height: 5, cursor: 0, start: 0, wantStart: 0, wantEnd: 5},
		{name: "cursor at bottom scrolls", n: 10, height: 5, cursor: 9, start: 0, wantStart: 5, wantEnd: 10},
		{name: "cursor in middle keeps start", n: 10, height: 5, cursor: 2, start: 0, wantStart: 0, wantEnd: 5},
		{name: "cursor above start recenters", n: 10, height: 5, cursor: 1, start: 4, wantStart: 1, wantEnd: 6},
		{name: "cursor below window", n: 10, height: 5, cursor: 7, start: 0, wantStart: 3, wantEnd: 8},
		{name: "stale start clamped", n: 10, height: 5, cursor: 9, start: 99, wantStart: 5, wantEnd: 10},
		{name: "negative start clamped", n: 10, height: 5, cursor: 0, start: -3, wantStart: 0, wantEnd: 5},
		{name: "negative cursor clamped", n: 10, height: 5, cursor: -2, start: 0, wantStart: 0, wantEnd: 5},
		{name: "cursor beyond end clamped", n: 10, height: 5, cursor: 42, start: 0, wantStart: 5, wantEnd: 10},
		{name: "no items", n: 0, height: 5, cursor: 0, start: 0, wantStart: 0, wantEnd: 0},
		{name: "no height", n: 10, height: 0, cursor: 0, start: 0, wantStart: 0, wantEnd: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := Window(tt.n, tt.height, tt.cursor, tt.start)
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Fatalf("Window(%d, %d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.n, tt.height, tt.cursor, tt.start, gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
			if gotEnd < gotStart {
				t.Fatalf("end %d before start %d", gotEnd, gotStart)
			}
			if gotStart < 0 || gotEnd > tt.n {
				t.Fatalf("window [%d, %d) outside [0, %d)", gotStart, gotEnd, tt.n)
			}
			if tt.cursor >= 0 && tt.cursor < tt.n && tt.height > 0 && tt.n > 0 {
				if tt.cursor < gotStart || tt.cursor >= gotEnd {
					t.Fatalf("cursor %d not visible in [%d, %d)", tt.cursor, gotStart, gotEnd)
				}
			}
		})
	}
}

func TestLabelsFor(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		want LabelMode
	}{
		{name: "full at 80x24 pane", w: 34, h: 20, want: LabelsFull},
		{name: "full at boundary", w: 30, h: 10, want: LabelsFull},
		{name: "markers when short", w: 34, h: 9, want: LabelsMarker},
		{name: "markers when narrow", w: 29, h: 20, want: LabelsMarker},
		{name: "markers at boundary", w: 12, h: 4, want: LabelsMarker},
		{name: "none below width", w: 11, h: 10, want: LabelsNone},
		{name: "none below height", w: 30, h: 3, want: LabelsNone},
		{name: "none tiny", w: 0, h: 0, want: LabelsNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LabelsFor(tt.w, tt.h); got != tt.want {
				t.Fatalf("LabelsFor(%d, %d) = %v, want %v", tt.w, tt.h, got, tt.want)
			}
		})
	}
}

func TestTableColumns(t *testing.T) {
	keys := func(cols []Column) []string {
		out := make([]string, len(cols))
		for i, c := range cols {
			out[i] = c.Key
		}
		return out
	}
	tests := []struct {
		name  string
		width int
		want  []string
	}{
		{
			name: "everything fits", width: 60,
			want: []string{"hex", "call", "type", "alt", "gs", "track", "squawk", "dst", "dir"},
		},
		{
			name: "drops least important first", width: 40,
			want: []string{"hex", "call", "alt", "squawk", "dst"},
		},
		{
			name: "keeps one column for tiny panes", width: 4,
			want: []string{"hex"},
		},
		{name: "zero width", width: 0, want: nil},
		{name: "negative width", width: -10, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TableColumns(tt.width)
			gotKeys := keys(got)
			if len(gotKeys) != len(tt.want) {
				t.Fatalf("TableColumns(%d) keys = %v, want %v", tt.width, gotKeys, tt.want)
			}
			for i := range tt.want {
				if gotKeys[i] != tt.want[i] {
					t.Fatalf("TableColumns(%d) keys = %v, want %v", tt.width, gotKeys, tt.want)
				}
			}
			if tt.width > 0 && len(got) == 0 {
				t.Fatalf("TableColumns(%d) returned no columns", tt.width)
			}
		})
	}
}

func TestNewPolar(t *testing.T) {
	tests := []struct {
		name        string
		w, h        int
		wantCenterR float64
		wantCenterC float64
		wantRowRad  float64
		wantColRad  float64
	}{
		{name: "wide grid uses horizontal radius", w: 41, h: 21,
			wantCenterR: 10, wantCenterC: 20, wantRowRad: 10, wantColRad: 20},
		{name: "short grid clamps vertical radius", w: 41, h: 6,
			wantCenterR: 2.5, wantCenterC: 20, wantRowRad: 2.5, wantColRad: 5},
		{name: "tall grid keeps the halved row radius", w: 21, h: 41,
			wantCenterR: 20, wantCenterC: 10, wantRowRad: 5, wantColRad: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPolar(tt.w, tt.h, 120)
			if p.CenterRow != tt.wantCenterR || p.CenterCol != tt.wantCenterC {
				t.Errorf("center = (%v, %v), want (%v, %v)",
					p.CenterRow, p.CenterCol, tt.wantCenterR, tt.wantCenterC)
			}
			if p.RowRadius != tt.wantRowRad || p.ColRadius != tt.wantColRad {
				t.Errorf("radius = (%v, %v), want (%v, %v)",
					p.RowRadius, p.ColRadius, tt.wantRowRad, tt.wantColRad)
			}
			if p.RangeNm != 120 {
				t.Errorf("RangeNm = %v, want 120", p.RangeNm)
			}
			if p.RowRadius > p.ColRadius {
				t.Errorf("row radius %v exceeds col radius %v", p.RowRadius, p.ColRadius)
			}
		})
	}
}

func TestPolarRangeBearing(t *testing.T) {
	p := NewPolar(41, 21, 120)
	tests := []struct {
		name      string
		dst       float64
		bearing   float64
		zeroRange bool // run against a projection with RangeNm 0
		wantRow   int
		wantCol   int
		wantOK    bool
	}{
		{name: "centre", dst: 0, bearing: 0, wantRow: 10, wantCol: 20, wantOK: true},
		{name: "due north half range", dst: 60, bearing: 0, wantRow: 5, wantCol: 20, wantOK: true},
		{name: "due east half range", dst: 60, bearing: 90, wantRow: 10, wantCol: 30, wantOK: true},
		{name: "due south full range", dst: 120, bearing: 180, wantRow: 20, wantCol: 20, wantOK: true},
		{name: "due west quarter range", dst: 30, bearing: 270, wantRow: 10, wantCol: 15, wantOK: true},
		{name: "beyond range", dst: 120.01, bearing: 45, wantOK: false},
		{name: "negative distance", dst: -1, bearing: 0, wantOK: false},
		{name: "not a number", dst: math.NaN(), bearing: 0, wantOK: false},
		{name: "zero range disables", dst: 0, bearing: 0, zeroRange: true, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pp := p
			if tt.zeroRange {
				pp.RangeNm = 0
			}
			row, col, ok := pp.RangeBearing(tt.dst, tt.bearing)
			if ok != tt.wantOK {
				t.Fatalf("RangeBearing(%v, %v) ok = %v, want %v", tt.dst, tt.bearing, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if row != tt.wantRow || col != tt.wantCol {
				t.Fatalf("RangeBearing(%v, %v) = (%d, %d), want (%d, %d)",
					tt.dst, tt.bearing, row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestPolarProject(t *testing.T) {
	p := NewPolar(41, 21, 120)
	tests := []struct {
		name                       string
		lat, lon, recvLat, recvLon float64
		wantOK                     bool
	}{
		{name: "due north of receiver", lat: 49, lon: 11, recvLat: 48, recvLon: 11, wantOK: true},
		{name: "receiver position invalid", lat: 49, lon: 11, recvLat: 91, recvLon: 11, wantOK: false},
		{name: "aircraft position invalid", lat: 91, lon: 11, recvLat: 48, recvLon: 11, wantOK: false},
		{name: "aircraft beyond range", lat: 10, lon: 11, recvLat: 48, recvLon: 11, wantOK: false},
		{name: "same point", lat: 48, lon: 11, recvLat: 48, recvLon: 11, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, ok := p.Project(tt.lat, tt.lon, tt.recvLat, tt.recvLon)
			if ok != tt.wantOK {
				t.Fatalf("Project(%v, %v, %v, %v) ok = %v, want %v",
					tt.lat, tt.lon, tt.recvLat, tt.recvLon, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if row < 0 || row > 20 || col < 0 || col > 40 {
				t.Fatalf("projected cell (%d, %d) outside the 41x21 grid", row, col)
			}
		})
	}
	// A point due north must land above the centre row; due east, right
	// of the centre column.
	if row, _, ok := p.Project(49, 11, 48, 11); !ok || row >= 10 {
		t.Fatalf("north target row = %d (ok=%v), want < 10", row, ok)
	}
	if _, col, ok := p.Project(48, 12, 48, 11); !ok || col <= 20 {
		t.Fatalf("east target col = %d (ok=%v), want > 20", col, ok)
	}
}

func TestHaversineNm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		want                   float64
		tolerance              float64
	}{
		{name: "same point", lat1: 48.2, lon1: 11.5, lat2: 48.2, lon2: 11.5, want: 0, tolerance: 1e-9},
		{name: "one degree of latitude", lat1: 48, lon1: 11, lat2: 49, lon2: 11, want: 60.04, tolerance: 0.05},
		{name: "one degree of longitude at the equator", lat1: 0, lon1: 0, lat2: 0, lon2: 1, want: 60.04, tolerance: 0.05},
		{name: "longitude shrinks with latitude", lat1: 60, lon1: 0, lat2: 60, lon2: 1, want: 30.02, tolerance: 0.05},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HaversineNm(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(got-tt.want) > tt.tolerance {
				t.Fatalf("HaversineNm(%v,%v,%v,%v) = %.3f, want %.3f±%.3f",
					tt.lat1, tt.lon1, tt.lat2, tt.lon2, got, tt.want, tt.tolerance)
			}
		})
	}
	// The formula is symmetric.
	a := HaversineNm(48.2, 11.5, 47.9, 11.2)
	b := HaversineNm(47.9, 11.2, 48.2, 11.5)
	if math.Abs(a-b) > 1e-9 {
		t.Fatalf("haversine not symmetric: %.9f vs %.9f", a, b)
	}
}

func TestBearingDeg(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		want                   float64
	}{
		{name: "north", lat1: 0, lon1: 0, lat2: 1, lon2: 0, want: 0},
		{name: "east", lat1: 0, lon1: 0, lat2: 0, lon2: 1, want: 90},
		{name: "south", lat1: 1, lon1: 0, lat2: 0, lon2: 0, want: 180},
		{name: "west", lat1: 0, lon1: 1, lat2: 0, lon2: 0, want: 270},
		{name: "north east", lat1: 0, lon1: 0, lat2: 1, lon2: 1, want: 45},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BearingDeg(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			diff := math.Abs(got - tt.want)
			if diff > 0.5 && diff < 359.5 {
				t.Fatalf("BearingDeg(%v,%v,%v,%v) = %.2f, want %.2f",
					tt.lat1, tt.lon1, tt.lat2, tt.lon2, got, tt.want)
			}
		})
	}
}
