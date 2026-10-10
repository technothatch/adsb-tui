// Command adsb-tui is a live terminal radar for ADS-B receivers
// running dump1090 or readsb. It reads the decoder's aircraft.json and
// receiver.json over HTTP and draws a polar range display beside a
// sortable detail table.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/technothatch/adsb-tui/internal/client"
	"github.com/technothatch/adsb-tui/internal/model"
	"github.com/technothatch/adsb-tui/internal/render"
)

// Exit codes: success, a runtime failure and a command-line usage
// error, in that order.
const (
	exitOK    = 0
	exitErr   = 1
	exitUsage = 2
)

// usageHeader introduces the flag list in help and error output.
const usageHeader = `usage: adsb-tui [flags]

Live terminal ADS-B radar for dump1090 and readsb: a polar range
display beside a sortable detail table.`

// Command-line defaults. The URL also honours two environment
// variables — ADSB_TUI_URL (TUI-specific) and DUMP1090_URL (shared
// with dump1090-cli and the wider ecosystem) — which take precedence
// over the built-in localhost default but not over the -url flag.
const (
	defaultURL     = "http://localhost:8080"
	defaultRangeNm = 120.0
	defaultRefresh = time.Second
	defaultTimeout = 5 * time.Second
)

// config is the validated command line.
type config struct {
	// url is the decoder origin, scheme://host[:port].
	url string
	// rangeNm is the ground distance at the outer ring.
	rangeNm float64
	// refresh is the data refresh interval.
	refresh time.Duration
	// theme is "dark" or "light".
	theme string
	// timeout bounds each HTTP request.
	timeout time.Duration
}

// parseArgs parses args (without the program name), consulting getenv
// for defaults. It returns the configuration, the full usage text for
// help and error output, and the parse or validation error. flag.ErrHelp
// is returned for -h.
func parseArgs(args []string, getenv func(string) string) (config, string, error) {
	var cfg config
	fs := flag.NewFlagSet("adsb-tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.url, "url", envBaseURL(getenv),
		"decoder base URL, scheme://host[:port] (env $ADSB_TUI_URL, else $DUMP1090_URL)")
	fs.Float64Var(&cfg.rangeNm, "range", defaultRangeNm,
		"plot range in nautical miles")
	fs.DurationVar(&cfg.refresh, "refresh", defaultRefresh,
		"interval between aircraft.json refreshes")
	fs.StringVar(&cfg.theme, "theme", "dark",
		"colour theme: dark or light")
	fs.DurationVar(&cfg.timeout, "timeout", defaultTimeout,
		"HTTP request timeout")

	err := fs.Parse(args)

	var defs bytes.Buffer
	fs.SetOutput(&defs)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
	usage := usageHeader + "\n\nflags:\n" + defs.String()
	if err != nil {
		return cfg, usage, err
	}
	if fs.NArg() > 0 {
		return cfg, usage, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	switch {
	case cfg.rangeNm <= 0:
		return cfg, usage, errors.New("-range must be positive")
	case cfg.refresh <= 0:
		return cfg, usage, errors.New("-refresh must be positive")
	case cfg.timeout <= 0:
		return cfg, usage, errors.New("-timeout must be positive")
	}
	if _, err := render.ByName(cfg.theme); err != nil {
		return cfg, usage, err
	}
	if err := validBaseURL(cfg.url); err != nil {
		return cfg, usage, err
	}
	return cfg, usage, nil
}

// validBaseURL rejects URLs the HTTP client could not use, so a typo
// fails at start-up instead of on every refresh.
func validBaseURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("-url must be an http:// or https:// origin, got %q", raw)
	}
	return nil
}

// envOr reads an environment variable, falling back when it is unset
// or blank.
func envOr(getenv func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return fallback
}

// envBaseURL resolves the default decoder origin from the environment.
// The TUI-specific ADSB_TUI_URL wins over the ecosystem-wide
// DUMP1090_URL (shared with dump1090-cli); both fall back to the
// built-in localhost origin.
func envBaseURL(getenv func(string) string) string {
	return envOr(getenv, "ADSB_TUI_URL", envOr(getenv, "DUMP1090_URL", defaultURL))
}

// main runs the program with the process arguments and exits with the
// resulting code.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

// run executes the program with the given arguments and streams and
// returns the process exit code.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	cfg, usage, err := parseArgs(args, getenv)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stdout, usage)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "adsb-tui: %s\n", err)
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	theme, err := render.ByName(cfg.theme)
	if err != nil {
		fmt.Fprintf(stderr, "adsb-tui: %s\n", err)
		return exitUsage
	}

	src := client.NewClient(cfg.url, cfg.timeout)
	recv, hasRecv, warn := fetchReceiver(context.Background(), src, cfg.timeout)

	m := model.New(model.Options{
		Source:       src,
		Receiver:     recv,
		HasReceiver:  hasRecv,
		ReceiverWarn: warn,
		RangeNm:      cfg.rangeNm,
		Theme:        theme,
		Refresh:      cfg.refresh,
	})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(stderr, "adsb-tui: %s\n", err)
		return exitErr
	}
	return exitOK
}

// fetchReceiver reads receiver.json once at start-up. A missing
// document or one without a position is not fatal: the polar plot is
// disabled and the reason is reported in the footer instead.
func fetchReceiver(ctx context.Context, src *client.Client, timeout time.Duration) (client.Receiver, bool, string) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	recv, err := src.FetchReceiver(ctx)
	switch {
	case err != nil:
		return recv, false, "receiver position unknown - table only"
	case !recv.HasPosition():
		return recv, false, "receiver.json has no position - table only"
	}
	return recv, true, ""
}
