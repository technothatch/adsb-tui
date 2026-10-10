package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// goodDoc is a minimal aircraft.json document.
const goodDoc = `{"now":1728000000.5,"messages":1234,"aircraft":[
  {"hex":"3c673a","flight":"DLH441  ","alt_baro":34975,"gs":462,"track":95,"squawk":"1013"},
  {"hex":"4b18d5","alt_baro":"ground"}]}`

// goodReceiver is a receiver.json document with a position.
const goodReceiver = `{"version":"readsb 4.0.0","refresh":1,"lat":48.2,"lon":11.5}`

// docServer answers only the paths listed as OK with body, everything
// else with 404.
func docServer(t *testing.T, okPaths map[string]string, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := okPaths[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
}

func TestFetchAircraftPathFallback(t *testing.T) {
	tests := []struct {
		name     string
		primary  int // status the /data/aircraft.json endpoint answers with (0 = serves)
		okPaths  []string
		wantPath string
	}{
		{name: "primary /data path", primary: 0, okPaths: []string{"/data/aircraft.json", "/aircraft.json", "/"}, wantPath: "/data/aircraft.json"},
		{name: "500 on primary still falls back", primary: http.StatusInternalServerError, okPaths: []string{"/aircraft.json", "/"}, wantPath: "/aircraft.json"},
		{name: "readsb API port serves the list at /", primary: http.StatusNotFound, okPaths: []string{"/"}, wantPath: "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok := map[string]bool{}
			for _, p := range tt.okPaths {
				ok[p] = true
			}
			served := ""
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/data/aircraft.json" && tt.primary != 0 {
					http.Error(w, "boom", tt.primary)
					return
				}
				if ok[r.URL.Path] {
					served = r.URL.Path
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(goodDoc))
					return
				}
				http.NotFound(w, r)
			}))
			defer ts.Close()

			c := NewClient(ts.URL, time.Second)
			doc, err := c.FetchAircraft(context.Background())
			if err != nil {
				t.Fatalf("FetchAircraft: %v", err)
			}
			if served != tt.wantPath {
				t.Errorf("served from %q, want %q", served, tt.wantPath)
			}
			if len(doc.Aircraft) != 2 {
				t.Fatalf("aircraft = %d, want 2", len(doc.Aircraft))
			}
			if doc.Aircraft[0].Hex != "3c673a" {
				t.Errorf("first hex = %q, want 3c673a", doc.Aircraft[0].Hex)
			}
			if doc.Messages != 1234 {
				t.Errorf("messages = %d, want 1234", doc.Messages)
			}
			if alt, ok := doc.Aircraft[1].AltFeet(); !ok || alt != 0 {
				t.Errorf("ground alt = (%v, %v), want (0, true)", alt, ok)
			}
		})
	}
}

func TestFetchAircraftRejectsNonDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "html error page", body: "<html><body>502 Bad Gateway</body></html>"},
		{name: "json without aircraft array", body: `{"now":1}`},
		{name: "truncated json", body: `{"aircraft":[`},
		{name: "json array instead of object", body: `[{"hex":"abc123"}]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := docServer(t, map[string]string{
				"/data/aircraft.json": "",
				"/aircraft.json":      "",
				"/":                   "",
			}, tt.body)
			defer ts.Close()

			c := NewClient(ts.URL, time.Second)
			if _, err := c.FetchAircraft(context.Background()); err == nil {
				t.Fatal("FetchAircraft accepted a non-document")
			} else if !strings.Contains(err.Error(), "no usable response") {
				t.Errorf("error = %q, want it to mention no usable response", err)
			}
		})
	}
}

func TestFetchAircraftSendsHeaders(t *testing.T) {
	var gotUA, gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(goodDoc))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, time.Second)
	if _, err := c.FetchAircraft(context.Background()); err != nil {
		t.Fatalf("FetchAircraft: %v", err)
	}
	if gotUA == "" {
		t.Error("User-Agent header not set")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
}

func TestFetchReceiver(t *testing.T) {
	tests := []struct {
		name    string
		okPath  string
		wantPos bool
		wantLat float64
		wantErr bool
	}{
		{name: "with position", okPath: "/data/receiver.json", wantPos: true, wantLat: 48.2},
		{name: "falls back to root path", okPath: "/receiver.json", wantPos: true, wantLat: 48.2},
		{name: "document without position", okPath: "/data/receiver.json", wantPos: false},
		{name: "404 everywhere", okPath: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := goodReceiver
			if !tt.wantPos && !tt.wantErr {
				body = `{"version":"dump1090-fa 3.8.1","refresh":1}`
			}
			var okPaths map[string]string
			if tt.okPath != "" {
				okPaths = map[string]string{tt.okPath: ""}
			}
			ts := docServer(t, okPaths, body)
			defer ts.Close()

			c := NewClient(ts.URL, time.Second)
			recv, err := c.FetchReceiver(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Fatal("FetchReceiver wanted an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("FetchReceiver: %v", err)
			}
			if recv.HasPosition() != tt.wantPos {
				t.Fatalf("HasPosition = %v, want %v", recv.HasPosition(), tt.wantPos)
			}
			if tt.wantPos {
				if recv.Lat == nil || *recv.Lat != tt.wantLat {
					t.Fatalf("lat = %v, want %v", recv.Lat, tt.wantLat)
				}
				if recv.Lon == nil || *recv.Lon != 11.5 {
					t.Fatalf("lon = %v, want 11.5", recv.Lon)
				}
			}
		})
	}
}

func TestFetchReceiverRejectsArrays(t *testing.T) {
	ts := docServer(t, map[string]string{"/data/receiver.json": ""}, `["not","an","object"]`)
	defer ts.Close()

	c := NewClient(ts.URL, time.Second)
	if _, err := c.FetchReceiver(context.Background()); err == nil {
		t.Fatal("FetchReceiver accepted a JSON array")
	}
}

func TestFetchTransportError(t *testing.T) {
	ts := docServer(t, map[string]string{"/data/aircraft.json": ""}, goodDoc)
	c := NewClient(ts.URL, time.Second)
	ts.Close() // nothing listens any more

	if _, err := c.FetchAircraft(context.Background()); err == nil {
		t.Fatal("FetchAircraft wanted a transport error, got none")
	}
}

func TestBaseURLNormalisation(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "http://localhost:8080", want: "http://localhost:8080"},
		{raw: "http://localhost:8080/", want: "http://localhost:8080"},
		{raw: " http://readsb.lan:8080 ", want: "http://readsb.lan:8080"},
		{raw: "https://feed.example.com//", want: "https://feed.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			c := NewClient(tt.raw, time.Second)
			if got := c.BaseURL(); got != tt.want {
				t.Fatalf("NewClient(%q).BaseURL() = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestStatusText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{code: http.StatusNotFound, want: "404 Not Found"},
		{code: http.StatusInternalServerError, want: "500 Internal Server Error"},
		{code: 599, want: "HTTP 599"},
	}
	for _, tt := range tests {
		if got := statusText(tt.code); got != tt.want {
			t.Errorf("statusText(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
