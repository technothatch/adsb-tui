package client

import (
	"encoding/json"
	"testing"
)

// acJSON unmarshals one aircraft.json element, failing the test on error.
func acJSON(t *testing.T, raw string) Aircraft {
	t.Helper()
	var a Aircraft
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return a
}

// fp returns a pointer to v.
func fp(v float64) *float64 { return &v }

func TestAltFeet(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   float64
		wantOK bool
	}{
		{name: "numeric altitude", raw: `{"hex":"abc123","alt_baro":34975}`, want: 34975, wantOK: true},
		{name: "negative altitude", raw: `{"hex":"abc123","alt_baro":-150}`, want: -150, wantOK: true},
		{name: "ground", raw: `{"hex":"abc123","alt_baro":"ground"}`, want: 0, wantOK: true},
		{name: "absent", raw: `{"hex":"abc123"}`, wantOK: false},
		{name: "null", raw: `{"hex":"abc123","alt_baro":null}`, wantOK: false},
		{name: "unknown string", raw: `{"hex":"abc123","alt_baro":"fl240"}`, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := acJSON(t, tt.raw).AltFeet()
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Fatalf("AltFeet(%s) = (%v, %v), want (%v, %v)", tt.raw, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestOnGround(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "ground", raw: `{"hex":"abc123","alt_baro":"ground"}`, want: true},
		{name: "airborne", raw: `{"hex":"abc123","alt_baro":350}`, want: false},
		{name: "absent", raw: `{"hex":"abc123"}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acJSON(t, tt.raw).OnGround(); got != tt.want {
				t.Fatalf("OnGround(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCallsignAndType(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantCall string
		wantType string
	}{
		{name: "padded flight trimmed", raw: `{"hex":"abc123","flight":"DLH441  ","type":"A20N"}`, wantCall: "DLH441", wantType: "A20N"},
		{name: "short flight kept", raw: `{"hex":"abc123","flight":"ABC1"}`, wantCall: "ABC1"},
		{name: "no callsign", raw: `{"hex":"abc123"}`, wantCall: ""},
		{name: "short type key fallback", raw: `{"hex":"abc123","t":"B738"}`, wantType: "B738"},
		{name: "type wins over t", raw: `{"hex":"abc123","type":"A320","t":"A319"}`, wantType: "A320"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := acJSON(t, tt.raw)
			if got := a.Callsign(); got != tt.wantCall {
				t.Errorf("Callsign() = %q, want %q", got, tt.wantCall)
			}
			if got := a.TypeName(); got != tt.wantType {
				t.Errorf("TypeName() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestIsEmergency(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "squawk 7500", raw: `{"hex":"abc123","squawk":"7500"}`, want: true},
		{name: "squawk 7600", raw: `{"hex":"abc123","squawk":"7600"}`, want: true},
		{name: "squawk 7700", raw: `{"hex":"abc123","squawk":"7700"}`, want: true},
		{name: "emergency classification only", raw: `{"hex":"abc123","emergency":"lifeguard"}`, want: true},
		{name: "unlawful diversion", raw: `{"hex":"abc123","squawk":"7500","emergency":"unlawful"}`, want: true},
		{name: "emergency none", raw: `{"hex":"abc123","emergency":"none"}`, want: false},
		{name: "ordinary squawk", raw: `{"hex":"abc123","squawk":"1013"}`, want: false},
		{name: "nothing set", raw: `{"hex":"abc123"}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acJSON(t, tt.raw).IsEmergency(); got != tt.want {
				t.Fatalf("IsEmergency(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestHasPosition(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "valid pair", raw: `{"hex":"abc123","lat":48.2,"lon":11.5}`, want: true},
		{name: "missing lon", raw: `{"hex":"abc123","lat":48.2}`, want: false},
		{name: "missing lat", raw: `{"hex":"abc123","lon":11.5}`, want: false},
		{name: "null", raw: `{"hex":"abc123","lat":null,"lon":null}`, want: false},
		{name: "latitude out of range", raw: `{"hex":"abc123","lat":91,"lon":11.5}`, want: false},
		{name: "longitude out of range", raw: `{"hex":"abc123","lat":48.2,"lon":181}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acJSON(t, tt.raw).HasPosition(); got != tt.want {
				t.Fatalf("HasPosition(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestReceiverHasPosition(t *testing.T) {
	tests := []struct {
		name string
		recv Receiver
		want bool
	}{
		{name: "with position", recv: Receiver{Lat: fp(48.2), Lon: fp(11.5)}, want: true},
		{name: "without position", recv: Receiver{}, want: false},
		{name: "lat only", recv: Receiver{Lat: fp(48.2)}, want: false},
		{name: "lon only", recv: Receiver{Lon: fp(11.5)}, want: false},
		{name: "out of range", recv: Receiver{Lat: fp(-91), Lon: fp(0)}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.recv.HasPosition(); got != tt.want {
				t.Fatalf("HasPosition() = %v, want %v", got, tt.want)
			}
		})
	}
}
