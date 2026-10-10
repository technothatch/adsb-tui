package model

import (
	"encoding/json"
	"testing"

	"github.com/technothatch/adsb-tui/internal/client"
)

// fp returns a pointer to v.
func fp(v float64) *float64 { return &v }

// entry builds an entry from a raw aircraft.json element.
func entry(t *testing.T, raw string) Entry {
	t.Helper()
	var a client.Aircraft
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return Entry{Ac: a}
}

// distEntry builds an entry with a known distance/bearing from the
// receiver (the state Prepare produces for a positioned aircraft).
func distEntry(hex, call string, dst, dir float64) Entry {
	return Entry{
		Ac:      client.Aircraft{Hex: hex, Flight: call},
		Dst:     dst,
		Dir:     dir,
		HasDist: true,
	}
}

func TestFilterEntries(t *testing.T) {
	entries := []Entry{
		entry(t, `{"hex":"3c673a","flight":"DLH441  "}`),
		entry(t, `{"hex":"4b18d5","flight":"IFY230  "}`),
		entry(t, `{"hex":"a1b2c3","flight":"        "}`),
	}
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "empty keeps all", query: "", want: 3},
		{name: "blank keeps all", query: "   ", want: 3},
		{name: "callsign prefix", query: "DLH", want: 1},
		{name: "callsign substring", query: "441", want: 1},
		{name: "case insensitive", query: "dlh441", want: 1},
		{name: "matches hex too", query: "4b18", want: 1},
		{name: "hex case insensitive", query: "3C673A", want: 1},
		{name: "no match", query: "ZZZ", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(entries, tt.query)
			if len(got) != tt.want {
				t.Fatalf("Filter(%q) kept %d entries, want %d", tt.query, len(got), tt.want)
			}
		})
	}
	// The original slice must not be mutated when filtering.
	if len(entries) != 3 {
		t.Fatalf("Filter mutated the input: %d entries", len(entries))
	}
}

func TestSortEntries(t *testing.T) {
	alt := func(e Entry, raw string) Entry {
		e.Ac.AltBaro = json.RawMessage(raw)
		return e
	}
	// neither has no position and no altitude; the rest carry both a
	// decoder distance and an altitude. Its callsign sorts last too, so
	// every sort mode must park it at the end.
	neither := entry(t, `{"hex":"zzzzzz","flight":"ZZZ999  "}`)
	a := alt(distEntry("aaaaaa", "UAL123  ", 3, 0), "35000")
	b := alt(distEntry("bbbbbb", "DLH441  ", 8, 90), "12000")
	c := alt(distEntry("cccccc", "SWR13F  ", 21, 180), "24500")

	tests := []struct {
		name  string
		key   SortKey
		strip bool // drop alt_baro from the unknown entry first
		want  []string
	}{
		{name: "distance nearest first", key: SortDistance,
			want: []string{"aaaaaa", "bbbbbb", "cccccc", "zzzzzz"}},
		{name: "callsign alphabetical", key: SortCallsign,
			want: []string{"bbbbbb", "cccccc", "aaaaaa", "zzzzzz"}},
		{name: "altitude highest first", key: SortAltitude, strip: true,
			want: []string{"aaaaaa", "cccccc", "bbbbbb", "zzzzzz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unknown := neither
			if tt.strip {
				unknown.Ac.AltBaro = nil // no altitude at all must sort last
			}
			entries := []Entry{unknown, a, b, c}
			Sort(entries, tt.key)
			got := make([]string, len(entries))
			for i, e := range entries {
				got[i] = e.Ac.Hex
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("Sort(%v) order = %v, want %v", tt.key, got, tt.want)
				}
			}
		})
	}
}

func TestPrepareDerivesDistance(t *testing.T) {
	recv := client.Receiver{Lat: fp(48.2), Lon: fp(11.5)}
	acs := []client.Aircraft{
		{Hex: "aaaaaa", Lat: fp(48.3), Lon: fp(11.5)}, // ~6 nm north
		{Hex: "bbbbbb"}, // no position, no dst
		{Hex: "cccccc", Dst: fp(42), Dir: fp(90)}, // decoder-supplied range
		{Hex: "dddddd", Dst: fp(-5), Dir: fp(0)},  // nonsense distance is dropped
	}
	entries := Prepare(acs, recv, true)
	if len(entries) != 4 {
		t.Fatalf("len = %d, want 4", len(entries))
	}
	if !entries[0].HasPos || !entries[0].HasDist {
		t.Fatalf("positioned entry: HasPos=%v HasDist=%v, want both true", entries[0].HasPos, entries[0].HasDist)
	}
	if entries[0].Dst < 5.9 || entries[0].Dst > 6.1 {
		t.Errorf("distance = %.3f, want ~6 nm", entries[0].Dst)
	}
	if d := entries[0].Dir; d > 1 && d < 359 {
		t.Errorf("bearing = %.3f, want ~0 (north)", d)
	}
	if entries[1].HasPos || entries[1].HasDist {
		t.Errorf("bare entry has distance: %+v", entries[1])
	}
	if !entries[2].HasDist || entries[2].Dst != 42 {
		t.Errorf("decoder dst not used: %+v", entries[2])
	}
	if entries[3].HasDist {
		t.Errorf("negative decoder dst accepted: %+v", entries[3])
	}
}

func TestPrepareWithoutReceiver(t *testing.T) {
	recv := client.Receiver{}
	acs := []client.Aircraft{
		{Hex: "aaaaaa", Lat: fp(48.3), Lon: fp(11.5)}, // position but no receiver
		{Hex: "bbbbbb", Dst: fp(10), Dir: fp(180)},    // decoder range wins
	}
	entries := Prepare(acs, recv, false)
	if entries[0].HasPos {
		t.Errorf("entry plotted without a receiver position: %+v", entries[0])
	}
	if !entries[1].HasDist || entries[1].Dst != 10 {
		t.Errorf("decoder dst not used: %+v", entries[1])
	}
}

func TestEntryTarget(t *testing.T) {
	e := entry(t, `{"hex":"abc123","flight":"TEST123 ","alt_baro":35000,"lat":48.3,"lon":11.6,"squawk":"7700"}`)
	e.HasPos = true
	e.HasDist = true
	e.Dst = 12
	e.Dir = 45

	tgt := e.Target(true)
	if tgt.Callsign != "TEST123" {
		t.Errorf("Callsign = %q, want TEST123", tgt.Callsign)
	}
	if tgt.Hex != "abc123" {
		t.Errorf("Hex = %q, want abc123", tgt.Hex)
	}
	if tgt.AltFeet != 35000 {
		t.Errorf("AltFeet = %v, want 35000", tgt.AltFeet)
	}
	if !tgt.Emergency {
		t.Error("7700 not flagged as emergency")
	}
	if !tgt.Selected {
		t.Error("selected flag not propagated")
	}
	if !tgt.HasPos || !tgt.HasDist || tgt.DstNm != 12 || tgt.DirDeg != 45 {
		t.Errorf("range fields wrong: %+v", tgt)
	}
	if tgt.Lat != 48.3 || tgt.Lon != 11.6 {
		t.Errorf("position = (%v, %v), want (48.3, 11.6)", tgt.Lat, tgt.Lon)
	}

	// A ground aircraft with no altitude must not invent one.
	gnd := entry(t, `{"hex":"abc123","alt_baro":"ground"}`)
	tgt = gnd.Target(false)
	if !tgt.OnGround {
		t.Error("ground flag missing")
	}
	if tgt.AltFeet != 0 {
		t.Errorf("AltFeet = %v, want 0", tgt.AltFeet)
	}
	if tgt.Selected {
		t.Error("selected set on unselected entry")
	}
	if tgt.Emergency {
		t.Error("ordinary aircraft flagged as emergency")
	}
	if tgt.HasPos || tgt.HasDist {
		t.Error("positionless entry reports position data")
	}
	if tgt.DstNm != 0 || tgt.DirDeg != 0 {
		t.Errorf("invented range data: dst=%v dir=%v", tgt.DstNm, tgt.DirDeg)
	}
}
