// Package model is the live application state behind the TUI: it owns
// the data feed, the filter/sort/cursor state and the composition of
// each frame. Rendering itself lives in package render; this package
// only decides what goes where.
package model

import (
	"math"
	"sort"
	"strings"

	"github.com/technothatch/adsb-tui/internal/client"
	"github.com/technothatch/adsb-tui/internal/layout"
	"github.com/technothatch/adsb-tui/internal/render"
)

// SortKey selects the table order. The digit keys 1/2/3 map onto these
// values in that order.
type SortKey int

// The three orders the table can be sorted by.
const (
	// SortDistance orders by range from the receiver, nearest first.
	SortDistance SortKey = iota
	// SortCallsign orders alphabetically by callsign.
	SortCallsign
	// SortAltitude orders by altitude, highest first.
	SortAltitude
)

// String returns the order's name as shown in the status line.
func (k SortKey) String() string {
	switch k {
	case SortCallsign:
		return "callsign"
	case SortAltitude:
		return "altitude"
	}
	return "distance"
}

// Entry is one aircraft enriched with the fields derived from the
// receiver position: ground distance and bearing from the station,
// plus whether the aircraft can be plotted on the polar display.
type Entry struct {
	// Ac is the raw decoder record.
	Ac client.Aircraft
	// Dst and Dir are the distance in nautical miles and the bearing
	// in degrees from the receiver.
	Dst, Dir float64
	// HasPos reports a position that can be projected against the
	// receiver position.
	HasPos bool
	// HasDist reports that Dst/Dir are known, either computed from
	// both positions or copied from the decoder's own values.
	HasDist bool
}

// Prepare derives the range-display fields for every aircraft. With a
// receiver position the great-circle numbers are computed locally;
// without one the decoder's own dst/dir pair is used when present, and
// an aircraft with neither is simply not plottable.
func Prepare(acs []client.Aircraft, recv client.Receiver, hasRecv bool) []Entry {
	out := make([]Entry, 0, len(acs))
	recvOK := hasRecv && recv.HasPosition()
	for _, a := range acs {
		e := Entry{Ac: a}
		switch {
		case recvOK && a.HasPosition():
			e.Dst = layout.HaversineNm(*recv.Lat, *recv.Lon, *a.Lat, *a.Lon)
			e.Dir = layout.BearingDeg(*recv.Lat, *recv.Lon, *a.Lat, *a.Lon)
			e.HasPos = true
			e.HasDist = true
		case a.Dst != nil && a.Dir != nil && !math.IsNaN(*a.Dst) && *a.Dst >= 0:
			e.Dst, e.Dir, e.HasDist = *a.Dst, *a.Dir, true
		}
		out = append(out, e)
	}
	return out
}

// Filter keeps the entries whose callsign or ICAO hex contains query,
// case-insensitively. An empty or blank query keeps everything.
func Filter(entries []Entry, query string) []Entry {
	q := strings.ToUpper(strings.TrimSpace(query))
	if q == "" {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		name := strings.ToUpper(e.Ac.Callsign())
		hex := strings.ToUpper(e.Ac.Hex)
		if strings.Contains(name, q) || strings.Contains(hex, q) {
			out = append(out, e)
		}
	}
	return out
}

// Sort orders entries in place. Missing values always sort last, and a
// stable sort keeps equal entries in decoder order.
func Sort(entries []Entry, key SortKey) {
	switch key {
	case SortCallsign:
		sort.SliceStable(entries, func(i, j int) bool {
			return lessName(sortName(entries[i]), sortName(entries[j]))
		})
	case SortAltitude:
		sort.SliceStable(entries, func(i, j int) bool {
			return sortAlt(entries[i]) > sortAlt(entries[j])
		})
	default:
		sort.SliceStable(entries, func(i, j int) bool {
			return sortDst(entries[i]) < sortDst(entries[j])
		})
	}
}

// Target converts an entry into a blip for the range display.
func (e Entry) Target(selected bool) render.Target {
	alt, hasAlt := e.Ac.AltFeet()
	if !hasAlt {
		alt = 0
	}
	lat, lon := 0.0, 0.0
	if e.Ac.Lat != nil {
		lat = *e.Ac.Lat
	}
	if e.Ac.Lon != nil {
		lon = *e.Ac.Lon
	}
	return render.Target{
		Callsign:  e.Ac.Callsign(),
		Hex:       e.Ac.Hex,
		Lat:       lat,
		Lon:       lon,
		HasPos:    e.HasPos,
		HasDist:   e.HasDist,
		DstNm:     e.Dst,
		DirDeg:    e.Dir,
		AltFeet:   alt,
		OnGround:  e.Ac.OnGround(),
		Emergency: e.Ac.IsEmergency(),
		Selected:  selected,
	}
}

// sortName is the callsign (falling back to the hex address) in upper
// case; sorting by a stable string keeps the display predictable.
func sortName(e Entry) string {
	if cs := strings.ToUpper(e.Ac.Callsign()); cs != "" {
		return cs
	}
	return strings.ToUpper(e.Ac.Hex)
}

// lessName compares two sortName results with empties last.
func lessName(a, b string) bool {
	if a == "" || b == "" {
		return a != "" && b == ""
	}
	return a < b
}

// sortAlt is the altitude in feet, -1 when the decoder reported none,
// so unknown aircraft trail the list in the descending order.
func sortAlt(e Entry) float64 {
	alt, ok := e.Ac.AltFeet()
	if !ok {
		return -1
	}
	return alt
}

// sortDst is the distance in nautical miles, +Inf when unknown, so
// unlocated aircraft trail the nearest-first order.
func sortDst(e Entry) float64 {
	if !e.HasDist {
		return math.Inf(1)
	}
	return e.Dst
}
