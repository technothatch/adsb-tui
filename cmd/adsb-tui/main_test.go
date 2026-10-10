package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/technothatch/adsb-tui/internal/client"
)

// envFunc adapts a map to the getenv signature parseArgs and run take.
func envFunc(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestParseArgsDefaults(t *testing.T) {
	cfg, usage, err := parseArgs(nil, envFunc(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.url != defaultURL {
		t.Errorf("url = %q, want %q", cfg.url, defaultURL)
	}
	if cfg.rangeNm != 120.0 {
		t.Errorf("range = %v, want 120", cfg.rangeNm)
	}
	if cfg.refresh != time.Second {
		t.Errorf("refresh = %v, want 1s", cfg.refresh)
	}
	if cfg.theme != "dark" {
		t.Errorf("theme = %q, want dark", cfg.theme)
	}
	if cfg.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", cfg.timeout)
	}
	for _, want := range []string{
		"usage: adsb-tui [flags]",
		"-url", "-range", "-refresh", "-theme", "-timeout",
		"ADSB_TUI_URL", "DUMP1090_URL",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage missing %q:\n%s", want, usage)
		}
	}
}

func TestParseArgsFlagOverrides(t *testing.T) {
	cfg, _, err := parseArgs([]string{
		"-url", "http://decoder:30005",
		"-range", "60",
		"-refresh", "2s",
		"-theme", "light",
		"-timeout", "9s",
	}, envFunc(nil))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.url != "http://decoder:30005" || cfg.rangeNm != 60 ||
		cfg.refresh != 2*time.Second || cfg.theme != "light" || cfg.timeout != 9*time.Second {
		t.Errorf("cfg = %+v, want every flag applied", cfg)
	}
}

func TestEnvBaseURL(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "no environment", want: defaultURL},
		{name: "dump1090 url", env: map[string]string{"DUMP1090_URL": "http://dump1090.local:8080"},
			want: "http://dump1090.local:8080"},
		{name: "adsb-tui url", env: map[string]string{"ADSB_TUI_URL": "http://tui:1234"},
			want: "http://tui:1234"},
		{name: "tui wins over dump1090", env: map[string]string{
			"ADSB_TUI_URL": "http://tui:1234", "DUMP1090_URL": "http://cli:8080"},
			want: "http://tui:1234"},
		{name: "blank values are ignored", env: map[string]string{
			"ADSB_TUI_URL": "  ", "DUMP1090_URL": " "},
			want: defaultURL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envBaseURL(envFunc(tt.env)); got != tt.want {
				t.Fatalf("envBaseURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseArgsFlagBeatsEnvironment(t *testing.T) {
	env := map[string]string{
		"ADSB_TUI_URL": "http://tui:1234",
		"DUMP1090_URL": "http://cli:8080",
	}
	cfg, _, err := parseArgs([]string{"-url", "http://flag:9999"}, envFunc(env))
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if cfg.url != "http://flag:9999" {
		t.Fatalf("url = %q, want the flag value", cfg.url)
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "negative range", args: []string{"-range=-5"}, want: "-range must be positive"},
		{name: "zero range", args: []string{"-range", "0"}, want: "-range must be positive"},
		{name: "zero refresh", args: []string{"-refresh", "0"}, want: "-refresh must be positive"},
		{name: "zero timeout", args: []string{"-timeout", "0"}, want: "-timeout must be positive"},
		{name: "unknown theme", args: []string{"-theme", "sepia"}, want: `unknown theme "sepia"`},
		{name: "ftp scheme", args: []string{"-url", "ftp://host"}, want: "-url must be an http:// or https://"},
		{name: "scheme missing", args: []string{"-url", "localhost:8080"}, want: "-url must be an http:// or https://"},
		{name: "blank url", args: []string{"-url", " "}, want: "-url must be an http:// or https://"},
		{name: "positional argument", args: []string{"extra"}, want: `unexpected argument "extra"`},
		{name: "unknown flag", args: []string{"-bogus"}, want: "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, usage, err := parseArgs(tt.args, envFunc(nil))
			if err == nil {
				t.Fatalf("parseArgs(%v) succeeded, want an error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
			if !strings.Contains(usage, "flags:") {
				t.Errorf("usage returned with the error is missing the flag list")
			}
		})
	}
}

func TestParseArgsHelp(t *testing.T) {
	_, usage, err := parseArgs([]string{"-h"}, envFunc(nil))
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("err = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(usage, "usage: adsb-tui [flags]") {
		t.Errorf("usage = %q, want the command header", usage)
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, &stdout, &stderr, envFunc(nil)); code != exitOK {
		t.Fatalf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout.String(), "usage: adsb-tui [flags]") {
		t.Errorf("stdout = %q, want the usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "bad range", args: []string{"-range", "0"}},
		{name: "bad theme", args: []string{"-theme", "sepia"}},
		{name: "bad url", args: []string{"-url", "localhost"}},
		{name: "unexpected argument", args: []string{"oops"}},
		{name: "unknown flag", args: []string{"-bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr, envFunc(nil)); code != exitUsage {
				t.Fatalf("exit = %d, want %d", code, exitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			out := stderr.String()
			if !strings.HasPrefix(out, "adsb-tui: ") {
				t.Errorf("stderr = %q, want the adsb-tui: prefix", out)
			}
			if !strings.Contains(out, "flags:") {
				t.Errorf("stderr = %q, want the flag list after the error", out)
			}
		})
	}
}

func TestValidBaseURL(t *testing.T) {
	tests := []struct {
		raw     string
		wantErr bool
	}{
		{raw: "http://localhost:8080"},
		{raw: "https://example.com/dump1090"},
		{raw: "  http://decoder:30005  "},
		{raw: "http://[::1]:8080"},
		{raw: "", wantErr: true},
		{raw: "localhost:8080", wantErr: true},
		{raw: "ftp://host", wantErr: true},
		{raw: "http://", wantErr: true},
		{raw: "://x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run("raw="+tt.raw, func(t *testing.T) {
			err := validBaseURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validBaseURL(%q) = nil, want an error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("validBaseURL(%q): %v", tt.raw, err)
			}
		})
	}
}

func TestFetchReceiver(t *testing.T) {
	const withPosition = `{"version":"readsb 4.0.0","refresh":1,"lat":48.2,"lon":11.5}`
	const withoutPosition = `{"version":"readsb 4.0.0"}`

	tests := []struct {
		name     string
		body     string
		status   int
		wantPos  bool
		wantErr  bool
		wantWarn string
	}{
		{name: "position present", body: withPosition, status: http.StatusOK,
			wantPos: true, wantWarn: ""},
		{name: "document without position", body: withoutPosition, status: http.StatusOK,
			wantPos: false, wantWarn: "receiver.json has no position - table only"},
		{name: "missing document", status: http.StatusNotFound,
			wantPos: false, wantErr: true, wantWarn: "receiver position unknown - table only"},
		{name: "malformed document", body: "not json at all", status: http.StatusOK,
			wantPos: false, wantErr: true, wantWarn: "receiver position unknown - table only"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/data/receiver.json" && r.URL.Path != "/receiver.json" {
					http.NotFound(w, r)
					return
				}
				if tt.status != http.StatusOK {
					w.WriteHeader(tt.status)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			src := client.NewClient(ts.URL, time.Second)
			recv, hasRecv, warn := fetchReceiver(context.Background(), src, time.Second)
			if hasRecv != tt.wantPos {
				t.Errorf("hasRecv = %v, want %v", hasRecv, tt.wantPos)
			}
			if warn != tt.wantWarn {
				t.Errorf("warn = %q, want %q", warn, tt.wantWarn)
			}
			if hasRecv && !recv.HasPosition() {
				t.Errorf("hasRecv reported true for receiver without position")
			}
		})
	}

	t.Run("decoder unreachable", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		base := ts.URL
		ts.Close() // nothing is listening any more

		src := client.NewClient(base, 100*time.Millisecond)
		recv, hasRecv, warn := fetchReceiver(context.Background(), src, time.Second)
		if hasRecv || recv.HasPosition() {
			t.Errorf("hasRecv = %v, want false against a dead decoder", hasRecv)
		}
		if want := "receiver position unknown - table only"; warn != want {
			t.Errorf("warn = %q, want %q", warn, want)
		}
	})
}
