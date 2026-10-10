# adsb-tui

[![CI](https://github.com/technothatch/adsb-tui/actions/workflows/ci.yml/badge.svg)](https://github.com/technothatch/adsb-tui/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/technothatch/adsb-tui/blob/main/LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/technothatch/adsb-tui)](https://github.com/technothatch/adsb-tui/releases)
[![Issues](https://img.shields.io/github/issues/technothatch/adsb-tui)](https://github.com/technothatch/adsb-tui/issues)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/technothatch/adsb-tui/pulls)
[![Code size](https://img.shields.io/github/languages/code-size/technothatch/adsb-tui)](https://github.com/technothatch/adsb-tui)
[![Last commit](https://img.shields.io/github/last-commit/technothatch/adsb-tui)](https://github.com/technothatch/adsb-tui/commits/main)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](https://go.dev/doc/devel/release)
[![Coverage](https://img.shields.io/badge/coverage-95%25-brightgreen.svg)](https://github.com/technothatch/adsb-tui)

**adsb-tui** is a live terminal radar for ADS-B receivers running
[dump1090](https://github.com/flightaware/dump1090) or
[readsb](https://github.com/wiedehopf/readsb). It polls the decoder's
`aircraft.json` and `receiver.json` over HTTP and draws a polar range display
beside a sortable, filterable detail table — a compact radarscope for the
aircraft your receiver is hearing, with no browser and no server-side setup.

## Features

- Polar range plot centred on the receiver position from `receiver.json`:
  range rings at 1/3, 2/3 and full scale, cardinal letters, and a radar sweep
  that keeps animating even while paused.
- Detail table with cursor (`j`/`k`, arrows, `pgup`/`pgdn`, `home`/`end`) and
  three sort orders: distance, callsign, altitude.
- Live filter with `/` — match by callsign or ICAO hex, applied with `enter`.
- Emergency awareness: squawk 7500/7600/7700 or a decoder `emergency`
  classification marks the row and the blip with `!`.
- Dark and light colour themes (`d`), both readable on an 80x24 terminal.
- Degrades gracefully: blip labels shrink to markers and then disappear as the
  pane shrinks; without a receiver position the polar plot turns off with a
  warning in the footer and the table still runs.
- Keyboard-driven alt-screen TUI; single static binary, pure Go, no cgo.

## Install

```sh
go install github.com/technothatch/adsb-tui/cmd/adsb-tui@latest
```

Or build from source:

```sh
git clone https://github.com/technothatch/adsb-tui.git
cd adsb-tui
make build        # or: go build -o bin/adsb-tui ./cmd/adsb-tui
```

## Quickstart

With no arguments `adsb-tui` reads `http://localhost:8080` — the default
dump1090-fa / readsb origin. Point it at your decoder and set the plot scale:

```sh
adsb-tui -url http://readsb.local:8080 -range 60
```

Every flag, straight from the binary (`adsb-tui -h`):

```text
usage: adsb-tui [flags]

Live terminal ADS-B radar for dump1090 and readsb: a polar range
display beside a sortable detail table.

flags:
  -range float
    	plot range in nautical miles (default 120)
  -refresh duration
    	interval between aircraft.json refreshes (default 1s)
  -theme string
    	colour theme: dark or light (default "dark")
  -timeout duration
    	HTTP request timeout (default 5s)
  -url string
    	decoder base URL, scheme://host[:port] (env $ADSB_TUI_URL, else $DUMP1090_URL) (default "http://localhost:8080")
```

The range pane, as rendered by the bundled plotter (`@` selected aircraft,
`!` emergency, `*` the rest):

```text
+- range 60 nm --------------------------------------------+
|                        .....N....                        |
|                    ....     |    ....                    |
|                  ..      ......      ..                  |
|                 .     ...   |  ...     :                 |
|                .    ..      |     ..::::.                |
|               .    .    ........ :::.    .               |
|               .   ..   ..   @:DLH441 350 .               |
|W--------------.---.----.----+-*-SWR13F 120--------------E|
|               .   ..   ..   |! EZY999 220.               |
|               .    ..   ..*.RYR2GK 310   .               |
|                .    ..      |     ..    .                |
|                 ..    ....  | ....    ..                 |
|                  ...     ......     ...                  |
|                    ....     |    ....                    |
|                        .....S....                        |
+----------------------------------------------------------+
```

The detail table beside it:

```text
 HEX    CALLSIGN    ALT    GS SQ       DST
-------------------------------------------
>3c673a DLH441    35000   462 1013    11.4
 4b18d5 SWR13F    12000   421 1000    19.2
!401234 EZY999    22000   388 7700    19.0
 4ca1f2 RYR2GK    31000   350 4572    19.7
```

Keys:

| key | action |
|-----|--------|
| `q`, `ctrl+c` | quit |
| `t` | cycle panes: split → range alone → table |
| `/` then `enter` / `esc` | filter by callsign or hex / cancel |
| `1` / `2` / `3` | sort by distance / callsign / altitude |
| `j` `k`, arrows, `pgup` `pgdn`, `home` `end` | move the cursor |
| `p` | pause refresh (the sweep keeps running) |
| `d` | switch colour theme |
| `h` | toggle the key reference |

## Configuration reference

| flag | environment | meaning | default |
|------|-------------|---------|---------|
| `-url` | `ADSB_TUI_URL`, else `DUMP1090_URL` | decoder base URL, `scheme://host[:port]`, no trailing path | `http://localhost:8080` |
| `-range` | — | ground distance at the outer ring, nautical miles | `120` |
| `-refresh` | — | interval between `aircraft.json` refreshes | `1s` |
| `-theme` | — | colour theme: `dark` or `light` | `dark` |
| `-timeout` | — | timeout for each HTTP request | `5s` |

`ADSB_TUI_URL` wins over `DUMP1090_URL` (shared with
[dump1090-cli](https://github.com/technothatch/dump1090-cli)); an explicit
`-url` beats both. `receiver.json` is read once at start-up — a missing
document or a missing position only disables the polar plot, never the table.

## How it works

- `cmd/adsb-tui` — flag parsing and validation, the one start-up
  `receiver.json` read (missing position → footer warning, table-only mode),
  and program wiring.
- `internal/client` — HTTP client for `aircraft.json` / `receiver.json` with
  a `/data/` → `/` path fallback, because dump1090-fa and readsb serve the
  documents at different paths.
- `internal/layout` — pure geometry: pane rectangles, the table columns that
  fit a given width, the viewport window, and the polar projection (its
  vertical radius is half the horizontal one to compensate for cell aspect).
- `internal/model` — the Bubble Tea state machine: fetch ticks, sweep
  animation, filter / sort / cursor, and the `View` that composes header,
  panes and footer.
- `internal/render` — the cell buffer plus the renderers for the polar plot,
  framed boxes, the table and the colour themes.

Per refresh the pipeline is: `fetch aircraft.json` → decode → `Prepare`
(derive distance and bearing per entry) → filter → sort → render. The only
dependencies are Bubble Tea and Lip Gloss.

## Development

```sh
git clone https://github.com/technothatch/adsb-tui.git
cd adsb-tui
make test     # go test -race -coverprofile coverage.out -covermode atomic ./...
make fmt      # gofmt -l (fails on any unformatted file)
make vet      # go vet ./...
make lint     # fmt + vet + golangci-lint (when installed)
```

Measured with `go test -coverprofile coverage.out -covermode atomic ./...`:
**94.8%** of statements across `cmd/adsb-tui`, `internal/client`,
`internal/layout`, `internal/model` and `internal/render`.

## Related projects

- [dump1090-fa](https://github.com/flightaware/dump1090) — the decoder this TUI usually talks to
- [readsb](https://github.com/wiedehopf/readsb) — a drop-in alternative decoder
- [tar1090](https://github.com/wiedehopf/tar1090) — web-based aircraft map for the same data
- [graphs1090](https://github.com/wiedehopf/graphs1090) — long-term statistics graphs

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 technothatch.
