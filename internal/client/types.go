// Package client fetches the JSON documents published by dump1090-fa and
// readsb: aircraft.json and receiver.json (see http://HOST/data/).
//
// Every field except hex is optional, decoders omit keys they do not
// know, and both are fine: a document only fails to decode when it is
// not valid JSON at all. Unknown keys from future decoder versions are
// ignored.
package client

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Receiver mirrors receiver.json: http://HOST/data/receiver.json
type Receiver struct {
	// Version is the decoder version string, e.g. "readsb 4.0.0".
	Version string `json:"version"`
	// Refresh is the configured JSON write interval in seconds.
	Refresh *float64 `json:"refresh"`
	// Lat and Lon are the receiver location. They are nil when the
	// decoder has no position configured (common for mobile setups).
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// HasPosition reports whether the receiver has a usable lat/lon pair.
// The polar plot depends on it: without a receiver position there is no
// center for the range display.
func (r Receiver) HasPosition() bool {
	return validLatLon(r.Lat, r.Lon)
}

// Doc mirrors aircraft.json: http://HOST/data/aircraft.json
type Doc struct {
	// Now is the decoder clock, in seconds since the Unix epoch.
	Now float64 `json:"now"`
	// Messages is the cumulative message count since the decoder
	// started.
	Messages int64 `json:"messages"`
	// Aircraft holds every currently tracked aircraft, in document
	// order.
	Aircraft []Aircraft `json:"aircraft"`
}

// Aircraft mirrors one element of aircraft.json. Every field except Hex
// is optional: a message that carries no position has no lat/lon, a
// purely Mode-S message may have no altitude, and so on.
type Aircraft struct {
	// Hex is the ICAO 24-bit address in lowercase hex, e.g. "3c673a".
	// Non-ICAO addresses (TIS-B and friends) carry a '~' prefix.
	Hex string `json:"hex"`
	// Flight is the callsign as received, still space-padded to eight
	// characters; see Callsign for the trimmed form.
	Flight string `json:"flight"`
	// Type is the aircraft type code from the decoder's database, e.g.
	// "A320"; T carries the same value under the shorter key some
	// feeds use.
	Type string `json:"type"`
	T    string `json:"t"`
	// R is the registration, e.g. "G-EUUU".
	R string `json:"r"`
	// Squawk is the 4-digit Mode A code as a string, e.g. "7700".
	Squawk string `json:"squawk"`
	// Emergency is the decoder's emergency classification ("none"
	// when there is none; otherwise e.g. "lifeguard", "unlawful").
	Emergency string `json:"emergency"`
	// Category is the emitter category, e.g. "A3".
	Category string `json:"category"`
	// AltBaro is the barometric altitude in feet or the literal string
	// "ground". It is raw JSON because of the two possible types; use
	// AltFeet or OnGround to read it.
	AltBaro json.RawMessage `json:"alt_baro"`
	// AltGeom is the geometric (GNSS) altitude in feet.
	AltGeom *float64 `json:"alt_geom"`
	// GS is ground speed in knots.
	GS *float64 `json:"gs"`
	// Track is the true track over ground in degrees.
	Track *float64 `json:"track"`
	// BaroRate is the vertical rate in feet per minute.
	BaroRate *float64 `json:"baro_rate"`
	// Lat and Lon are the aircraft position in decimal degrees, nil
	// when unknown.
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
	// Dst is the distance from the receiver in nautical miles and Dir
	// the bearing in degrees, when the decoder computed a receiver
	// position.
	Dst *float64 `json:"dst"`
	Dir *float64 `json:"dir"`
	// RSSI is the receive signal strength in dBFS (negative).
	RSSI *float64 `json:"rssi"`
	// Messages is the number of messages seen from this aircraft.
	Messages *int64 `json:"messages"`
	// Seen is the seconds since this aircraft last produced any
	// message.
	Seen *float64 `json:"seen"`
	// MLAT and TISB list the fields sourced from multilateration or
	// TIS-B, e.g. ["lat","lon"].
	MLAT []string `json:"mlat"`
	TISB []string `json:"tisb"`
}

// Callsign returns the trimmed callsign (aircraft.json pads flight to
// eight characters), or "" when none was received.
func (a Aircraft) Callsign() string {
	return strings.TrimSpace(a.Flight)
}

// TypeName returns the aircraft type code: the "type" key, falling back
// to the short "t" key some feeds use, or "" when the decoder has no
// database entry.
func (a Aircraft) TypeName() string {
	if a.Type != "" {
		return a.Type
	}
	return a.T
}

// AltFeet decodes AltBaro. The second return value is false when the
// field is absent or not understood; a value of "ground" decodes to 0
// feet with true, because the aircraft really is at field elevation.
func (a Aircraft) AltFeet() (float64, bool) {
	raw := trimJSONNull(a.AltBaro)
	if len(raw) == 0 {
		return 0, false
	}
	var feet float64
	if err := json.Unmarshal(raw, &feet); err == nil {
		return feet, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s == "ground" {
		return 0, true
	}
	return 0, false
}

// OnGround reports whether the decoder says the aircraft is on the
// ground (alt_baro "ground"). There is no other in-band signal in
// aircraft.json.
func (a Aircraft) OnGround() bool {
	if len(trimJSONNull(a.AltBaro)) == 0 {
		return false
	}
	var s string
	return json.Unmarshal(trimJSONNull(a.AltBaro), &s) == nil && s == "ground"
}

// HasPosition reports whether the aircraft carries a usable lat/lon pair.
func (a Aircraft) HasPosition() bool {
	return validLatLon(a.Lat, a.Lon)
}

// IsEmergency reports whether the decoder signalled an emergency: the
// squawk codes 7500/7600/7700 or any non-"none" emergency
// classification. Decoders often set only one of the two.
func (a Aircraft) IsEmergency() bool {
	switch a.Squawk {
	case "7500", "7600", "7700":
		return true
	}
	return a.Emergency != "" && a.Emergency != "none"
}

// validLatLon reports whether both pointers hold values inside the
// WGS84 degree ranges.
func validLatLon(lat, lon *float64) bool {
	if lat == nil || lon == nil {
		return false
	}
	return *lat >= -90 && *lat <= 90 && *lon >= -180 && *lon <= 180
}

// trimJSONNull returns raw without surrounding whitespace, or nil when
// the JSON value is absent or null.
func trimJSONNull(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	return trimmed
}
