package model

import (
	"context"
	"fmt"
	"math"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/technothatch/adsb-tui/internal/client"
	"github.com/technothatch/adsb-tui/internal/layout"
	"github.com/technothatch/adsb-tui/internal/render"
)

// Source is the live decoder feed. *client.Client implements it, and
// tests substitute httptest-backed doubles.
type Source interface {
	// FetchAircraft reads the current aircraft.json document.
	FetchAircraft(ctx context.Context) (client.Doc, error)
	// FetchReceiver reads receiver.json once at start-up.
	FetchReceiver(ctx context.Context) (client.Receiver, error)
}

// Timing and scale defaults. The sweep advances ten degrees every
// hundred milliseconds — a full turn in three seconds — and the data
// feed is re-read once per second unless configured otherwise.
const (
	sweepInterval  = 100 * time.Millisecond
	sweepStepDeg   = 10.0
	defaultRefresh = time.Second
	defaultRangeNm = 120.0
	fetchTimeout   = 5 * time.Second
)

// paneMode is the arrangement of the content area, cycled by the "t"
// key.
type paneMode int

// The three pane arrangements.
const (
	// paneSplit shows the range display beside the table.
	paneSplit paneMode = iota
	// paneMap shows the range display alone.
	paneMap
	// paneTable shows the table alone.
	paneTable
)

// String names the arrangement for the status line.
func (p paneMode) String() string {
	switch p {
	case paneMap:
		return "map"
	case paneTable:
		return "table"
	}
	return "split"
}

// next cycles to the following arrangement. Without a receiver
// position there is no polar plot to show, so the table is the only
// stop.
func (p paneMode) next(hasRecv bool) paneMode {
	if !hasRecv {
		return paneTable
	}
	switch p {
	case paneSplit:
		return paneMap
	case paneMap:
		return paneTable
	default:
		return paneSplit
	}
}

// Options configures New. A nil Source is allowed (the model then
// reports an error instead of fetching) but real runs always pass the
// decoder client.
type Options struct {
	// Source feeds the aircraft documents.
	Source Source
	// Receiver and HasReceiver carry the start-up receiver.json
	// position that centres the polar plot.
	Receiver    client.Receiver
	HasReceiver bool
	// ReceiverWarn is shown in the footer when the receiver position
	// is missing or unusable; the plot is then disabled.
	ReceiverWarn string
	// RangeNm is the ground distance at the outer ring.
	RangeNm float64
	// Theme is the starting colour theme (dark when empty).
	Theme render.Theme
	// Refresh is the data refresh interval (one second when zero).
	Refresh time.Duration
}

// Model holds every piece of state the TUI needs. It implements
// tea.Model with value receivers: Update works on a copy and returns
// it, which is how bubbletea models are conventionally written.
type Model struct {
	source    Source
	receiver  client.Receiver
	hasRecv   bool
	recvWarn  string
	rangeNm   float64
	refresh   time.Duration
	theme     render.Theme
	pane      paneMode
	paused    bool
	help      bool
	filtering bool
	filter    string
	draft     string
	sort      SortKey
	cursor    int
	offset    int
	sweep     float64
	size      layout.Size
	now       time.Time
	aircraft  []client.Aircraft
	entries   []Entry
	updated   time.Time
	err       string
	fetched   bool
}

// New builds a model from the options, filling in the defaults and
// starting in the arrangement the receiver position allows.
func New(opts Options) Model {
	m := Model{
		source:   opts.Source,
		receiver: opts.Receiver,
		recvWarn: opts.ReceiverWarn,
		rangeNm:  opts.RangeNm,
		refresh:  opts.Refresh,
		theme:    opts.Theme,
		pane:     paneTable,
		sort:     SortDistance,
	}
	m.hasRecv = opts.HasReceiver && opts.Receiver.HasPosition()
	if m.rangeNm <= 0 {
		m.rangeNm = defaultRangeNm
	}
	if m.refresh <= 0 {
		m.refresh = defaultRefresh
	}
	if m.theme.Name == "" {
		m.theme = render.Dark()
	}
	if m.hasRecv {
		m.pane = paneSplit
	}
	return m
}

// Init starts the sweep animation and the first data fetch.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.sweepTick(), m.fetchCmd())
}

// Update advances the state machine: window sizes, animation ticks,
// fetch results and key presses.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = layout.Size{W: msg.Width, H: msg.Height}
		m.clampScroll()
		return m, nil
	case sweepMsg:
		m.now = time.Time(msg)
		if !m.paused {
			m.sweep = math.Mod(m.sweep+sweepStepDeg, 360)
		}
		return m, m.sweepTick()
	case refreshMsg:
		m.now = time.Time(msg)
		if m.paused {
			return m, nil
		}
		return m, m.fetchCmd()
	case aircraftMsg:
		return m.applyAircraft(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// sweepMsg, refreshMsg and aircraftMsg are the private ticks and fetch
// results that drive the loop.
type (
	sweepMsg    time.Time
	refreshMsg  time.Time
	aircraftMsg struct {
		doc client.Doc
		err error
		at  time.Time
	}
)

// applyAircraft stores a fetch result and schedules the next refresh.
// A failed fetch keeps the previous data on screen and surfaces the
// error in the footer; a paused model does not reschedule.
func (m Model) applyAircraft(msg aircraftMsg) (tea.Model, tea.Cmd) {
	m.now = msg.at
	if msg.err != nil {
		m.err = msg.err.Error()
	} else {
		m.err = ""
		m.aircraft = msg.doc.Aircraft
		m.updated = msg.at
		m.fetched = true
	}
	m.rebuild()
	if m.paused {
		return m, nil
	}
	return m, m.scheduleRefresh()
}

// fetchCmd reads aircraft.json off the network. The nil-source guard
// keeps a headless model usable in tests that never wire a feed.
func (m Model) fetchCmd() tea.Cmd {
	src := m.source
	return func() tea.Msg {
		if src == nil {
			return aircraftMsg{err: fmt.Errorf("no data source configured"), at: time.Now()}
		}
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		doc, err := src.FetchAircraft(ctx)
		return aircraftMsg{doc: doc, err: err, at: time.Now()}
	}
}

// sweepTick schedules the next animation frame.
func (m Model) sweepTick() tea.Cmd {
	return tea.Tick(sweepInterval, func(t time.Time) tea.Msg { return sweepMsg(t) })
}

// scheduleRefresh schedules the next data refresh.
func (m Model) scheduleRefresh() tea.Cmd {
	return tea.Tick(m.refresh, func(t time.Time) tea.Msg { return refreshMsg(t) })
}

// rebuild re-derives, filters and sorts the visible entries, then
// keeps the cursor inside them.
func (m *Model) rebuild() {
	entries := Prepare(m.aircraft, m.receiver, m.hasRecv)
	entries = Filter(entries, m.filter)
	Sort(entries, m.sort)
	m.entries = entries
	m.clampScroll()
}

// clampScroll keeps the cursor in range and the viewport anchored so
// the selected row stays visible without jumping around on refresh.
func (m *Model) clampScroll() {
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	start, _ := layout.Window(len(m.entries), m.tableRowsHeight(), m.cursor, m.offset)
	m.offset = start
}

// emergencyCount is how many visible aircraft are in an emergency.
func (m Model) emergencyCount() int {
	n := 0
	for _, e := range m.entries {
		if e.Ac.IsEmergency() {
			n++
		}
	}
	return n
}
