package render

import (
	"strings"
	"testing"

	"github.com/technothatch/adsb-tui/internal/layout"
)

func tableFixture() TableOpts {
	return TableOpts{
		W:       30,
		Height:  6,
		Columns: layout.TableColumns(29),
		Rows: []TableRow{
			{Cells: map[string]string{"hex": "aaaaaa", "call": "UAL123", "alt": "35000", "squawk": "1000"},
				Selected: true},
			{Cells: map[string]string{"hex": "ff0001", "call": "RESCUE1", "alt": "GND", "squawk": "7700"},
				Emergency: true},
			{Cells: map[string]string{"hex": "cccccc", "call": "SWR13F"}},
		},
		Theme: Dark(),
	}
}

func TestTableLayout(t *testing.T) {
	o := tableFixture()
	got := layout.TableColumns(29)
	want := []string{"hex", "call", "alt", "squawk"}
	if len(got) != len(want) {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	for i, c := range want {
		if got[i].Key != c {
			t.Fatalf("column %d = %q, want %q (all: %v)", i, got[i].Key, c, got)
		}
	}
	if o.Theme.Name != "dark" {
		t.Fatalf("fixture theme = %q", o.Theme.Name)
	}
}

func TestTableRender(t *testing.T) {
	out := plainLines(Table(tableFixture()))
	if len(out) != 6 {
		t.Fatalf("Table returned %d lines, want 6", len(out))
	}
	for i, l := range out {
		if len(l) != 30 {
			t.Errorf("line %d is %d cells, want 30: %q", i, len(l), l)
		}
	}
	header, sep := out[0], out[1]
	if !strings.Contains(header, "HEX") || !strings.Contains(header, "CALLSIGN") ||
		!strings.Contains(header, "ALT") || !strings.Contains(header, "SQ") {
		t.Errorf("header %q missing column titles", header)
	}
	if strings.Trim(sep, "-") != "" {
		t.Errorf("separator %q is not all dashes", sep)
	}
	if got := out[2]; !strings.HasPrefix(got, ">") || !strings.Contains(got, "aaaaaa") ||
		!strings.Contains(got, "UAL123") {
		t.Errorf("selected row = %q", got)
	}
	if got := out[3]; !strings.HasPrefix(got, "!") || !strings.Contains(got, "ff0001") ||
		!strings.Contains(got, "RESCUE1") {
		t.Errorf("emergency row = %q", got)
	}
	if got := out[4]; !strings.HasPrefix(got, " ") || !strings.Contains(got, "cccccc") {
		t.Errorf("plain row = %q", got)
	}
	if got := out[5]; strings.TrimSpace(got) != "" {
		t.Errorf("unused row = %q, want blank", got)
	}
}

func TestTableSelectedEmergencyKeepsEmergencyMark(t *testing.T) {
	o := tableFixture()
	o.Height = 3
	o.Rows = []TableRow{{
		Cells:     map[string]string{"hex": "deadbe", "call": "HELP", "alt": "100", "squawk": "7700"},
		Selected:  true,
		Emergency: true,
	}}
	out := plainLines(Table(o))
	if !strings.HasPrefix(out[2], "!") {
		t.Fatalf("row = %q, want the ! emergency marker", out[2])
	}
}

func TestTableEmptyHint(t *testing.T) {
	o := tableFixture()
	o.Rows = nil
	o.Empty = "waiting for aircraft..."
	out := plainLines(Table(o))
	if len(out) != 6 {
		t.Fatalf("Table returned %d lines, want 6", len(out))
	}
	if !strings.Contains(out[2], "waiting for aircraft...") {
		t.Errorf("row 2 = %q, want the empty hint", out[2])
	}
	// The header stays visible even with no data.
	if !strings.Contains(out[0], "HEX") {
		t.Errorf("header = %q, want column titles", out[0])
	}
}

func TestTableClipsRowsToHeight(t *testing.T) {
	o := tableFixture()
	o.Height = 4 // header, separator, one row, nothing else
	out := plainLines(Table(o))
	if len(out) != 4 {
		t.Fatalf("Table returned %d lines, want 4", len(out))
	}
	if strings.Contains(strings.Join(out[2:], "\n"), "cccccc") {
		t.Errorf("row beyond the height was rendered: %v", out)
	}
}

func TestTableDegenerateSizes(t *testing.T) {
	tests := []struct {
		name string
		w, h int
	}{
		{name: "zero width", w: 0, h: 5},
		{name: "zero height", w: 30, h: 0},
		{name: "negative width", w: -1, h: 5},
		{name: "negative height", w: 30, h: -2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := tableFixture()
			o.W, o.Height = tt.w, tt.h
			if got := Table(o); got != nil {
				t.Fatalf("Table(w=%d, h=%d) = %v, want nil", tt.w, tt.h, got)
			}
		})
	}
}

func TestTableHeightOneRendersHeaderOnly(t *testing.T) {
	o := tableFixture()
	o.Height = 1
	out := plainLines(Table(o))
	if len(out) != 1 {
		t.Fatalf("Table returned %d lines, want 1", len(out))
	}
	if !strings.Contains(out[0], "HEX") {
		t.Errorf("header = %q", out[0])
	}
}
