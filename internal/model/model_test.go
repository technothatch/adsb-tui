package model

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/technothatch/adsb-tui/internal/client"
	"github.com/technothatch/adsb-tui/internal/render"
)

// stubSource is a Source double: no HTTP, no ports, instant answers.
type stubSource struct {
	doc   client.Doc
	err   error
	calls int
}

func (s *stubSource) FetchAircraft(context.Context) (client.Doc, error) {
	s.calls++
	return s.doc, s.err
}

func (s *stubSource) FetchReceiver(context.Context) (client.Receiver, error) {
	return client.Receiver{}, nil
}

// testReceiver is a receiver.json with a position (Munich-ish).
func testReceiver() client.Receiver {
	return client.Receiver{Lat: fp(48.2), Lon: fp(11.5)}
}

// testDoc is the fixture fleet: a north of the receiver (3 nm), b
// north-west (8.5 nm), c south-west (21.6 nm).
func testDoc() client.Doc {
	return client.Doc{
		Now:      1728000000,
		Messages: 4242,
		Aircraft: []client.Aircraft{
			{Hex: "aaaaaa", Flight: "UAL123  ", Lat: fp(48.25), Lon: fp(11.5),
				AltBaro: json.RawMessage("35000"), GS: fp(462), Track: fp(0), Squawk: "1000"},
			{Hex: "bbbbbb", Flight: "DLH441  ", Lat: fp(48.15), Lon: fp(11.3),
				AltBaro: json.RawMessage("12000"), GS: fp(380), Track: fp(240), Squawk: "7000"},
			{Hex: "cccccc", Flight: "SWR13F  ", Lat: fp(47.9), Lon: fp(11.2),
				AltBaro: json.RawMessage("24500"), GS: fp(400), Track: fp(210), Squawk: "1013"},
		},
	}
}

// newModel builds a receiver-backed model fed by the stub source.
func newModel(t *testing.T) Model {
	t.Helper()
	return New(Options{
		Source:      &stubSource{doc: testDoc()},
		Receiver:    testReceiver(),
		HasReceiver: true,
		RangeNm:     120,
		Theme:       render.Dark(),
		Refresh:     time.Second,
	})
}

// key builds a printable key press.
func key(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

var (
	enterKey = tea.KeyMsg{Type: tea.KeyEnter}
	escKey   = tea.KeyMsg{Type: tea.KeyEsc}
)

// upd feeds one message into the model and returns the new model.
func upd(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return mm
}

// updCmd is upd but also returns the command Update produced.
func updCmd(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return mm, cmd
}

// sized sets the window size.
func sized(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	return upd(t, m, tea.WindowSizeMsg{Width: w, Height: h})
}

// withData applies a successful fetch of the fixture fleet.
func withData(t *testing.T, m Model) Model {
	t.Helper()
	return upd(t, m, aircraftMsg{doc: testDoc(), at: time.Now()})
}

var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes SGR colour sequences so assertions see plain text.
func stripANSI(s string) string { return sgrPattern.ReplaceAllString(s, "") }

// checkFrame asserts the view is exactly h lines of w cells each and
// returns the stripped lines.
func checkFrame(t *testing.T, out string, w, h int) []string {
	t.Helper()
	if out == "" {
		t.Fatal("view is empty")
	}
	raw := strings.Split(out, "\n")
	if len(raw) != h {
		t.Fatalf("view has %d lines, want %d", len(raw), h)
	}
	lines := make([]string, h)
	for i, l := range raw {
		lines[i] = stripANSI(l)
		if got := len(lines[i]); got != w {
			t.Fatalf("line %d is %d cells, want %d: %q", i, got, w, lines[i])
		}
	}
	return lines
}

// hexOrder lists the visible entry hexes in order.
func hexOrder(m Model) []string {
	out := make([]string, len(m.entries))
	for i, e := range m.entries {
		out[i] = e.Ac.Hex
	}
	return out
}

func TestNewDefaultsAndReceiverGating(t *testing.T) {
	t.Run("with receiver starts split", func(t *testing.T) {
		m := newModel(t)
		if m.pane != paneSplit {
			t.Errorf("pane = %v, want split", m.pane)
		}
		if !m.hasRecv {
			t.Error("hasRecv = false with a positioned receiver")
		}
	})
	t.Run("without receiver starts table only", func(t *testing.T) {
		m := New(Options{Source: &stubSource{}})
		if m.pane != paneTable {
			t.Errorf("pane = %v, want table", m.pane)
		}
		if m.hasRecv {
			t.Error("hasRecv = true without a receiver")
		}
	})
	t.Run("receiver without position disables the plot", func(t *testing.T) {
		m := New(Options{Receiver: client.Receiver{}, HasReceiver: true})
		if m.hasRecv || m.pane != paneTable {
			t.Errorf("hasRecv=%v pane=%v, want false/table", m.hasRecv, m.pane)
		}
	})
	t.Run("zero options take defaults", func(t *testing.T) {
		m := New(Options{})
		if m.rangeNm != 120 {
			t.Errorf("rangeNm = %v, want 120", m.rangeNm)
		}
		if m.refresh != time.Second {
			t.Errorf("refresh = %v, want 1s", m.refresh)
		}
		if m.theme.Name != "dark" {
			t.Errorf("theme = %q, want dark", m.theme.Name)
		}
		if m.sort != SortDistance {
			t.Errorf("sort = %v, want distance", m.sort)
		}
	})
}

func TestInitSchedulesWork(t *testing.T) {
	if cmd := newModel(t).Init(); cmd == nil {
		t.Fatal("Init returned no command")
	}
}

func TestKeyQuit(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{name: "q", msg: key("q")},
		{name: "ctrl+c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(t)
			_, cmd := updCmd(t, m, tt.msg)
			if cmd == nil {
				t.Fatal("no quit command produced")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("command did not return tea.QuitMsg")
			}
		})
	}
}

func TestPauseToggle(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)
	if m.paused {
		t.Fatal("model starts paused")
	}

	// Pause: no refresh scheduled, header announces it.
	m, cmd := updCmd(t, m, key("p"))
	if !m.paused || cmd != nil {
		t.Fatalf("after p: paused=%v cmd=%v, want true/nil", m.paused, cmd)
	}
	if out := m.View(); !strings.Contains(stripANSI(out), "PAUSED") {
		t.Error("paused state not shown in the header")
	}

	// While paused neither refresh nor sweep are consumed.
	m2, cmd := updCmd(t, m, refreshMsg(time.Now()))
	if cmd != nil {
		t.Error("paused model scheduled a refresh")
	}
	if len(m2.entries) != 3 {
		t.Errorf("entries = %d, want 3", len(m2.entries))
	}
	m2, cmd = updCmd(t, m2, sweepMsg(time.Now()))
	if cmd == nil {
		t.Error("sweep tick not rescheduled (animation must keep ticking)")
	}
	if m2.sweep != 0 {
		t.Errorf("sweep advanced to %v while paused", m2.sweep)
	}

	// Unpause: refresh is scheduled again.
	m3, cmd := updCmd(t, m2, key("p"))
	if m3.paused || cmd == nil {
		t.Fatalf("after second p: paused=%v cmd=%v, want false/non-nil", m3.paused, cmd)
	}
	if out := m3.View(); strings.Contains(stripANSI(out), "PAUSED") {
		t.Error("PAUSED still shown after unpause")
	}
}

func TestSweepAnimation(t *testing.T) {
	m := newModel(t)
	if m.sweep != 0 {
		t.Fatalf("initial sweep = %v, want 0", m.sweep)
	}
	for i := 1; i <= 3; i++ {
		m, _ = updCmd(t, m, sweepMsg(time.Now()))
		want := float64(10 * i)
		if m.sweep != want {
			t.Fatalf("sweep after %d ticks = %v, want %v", i, m.sweep, want)
		}
	}
	// 36 ticks of 10 degrees wrap back to zero.
	for i := 4; i <= 36; i++ {
		m, _ = updCmd(t, m, sweepMsg(time.Now()))
	}
	if m.sweep != 0 {
		t.Fatalf("sweep after full turn = %v, want 0", m.sweep)
	}
	// Sweep ticks keep the animation alive by returning the next tick.
	if _, cmd := updCmd(t, m, sweepMsg(time.Now())); cmd == nil {
		t.Fatal("sweep tick not rescheduled")
	}
}

func TestPaneToggleWithReceiver(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	want := []paneMode{paneMap, paneTable, paneSplit}
	for i, w := range want {
		next, cmd := updCmd(t, m, key("t"))
		if cmd != nil {
			t.Errorf("step %d produced a command", i)
		}
		m = next
		if m.pane != w {
			t.Fatalf("step %d: pane = %v, want %v", i, m.pane, w)
		}
		if !strings.Contains(stripANSI(m.View()), "view "+w.String()) {
			t.Errorf("step %d: header missing %q", i, "view "+w.String())
		}
	}
}

func TestPaneToggleWithoutReceiver(t *testing.T) {
	m := New(Options{Source: &stubSource{doc: testDoc()}, HasReceiver: false,
		ReceiverWarn: "receiver position unknown - table only"})
	m = sized(t, m, 80, 24)
	for i := 0; i < 4; i++ {
		m, _ = updCmd(t, m, key("t"))
		if m.pane != paneTable {
			t.Fatalf("press %d: pane = %v, want table (no receiver, no plot)", i, m.pane)
		}
		out := stripANSI(m.View())
		if strings.Contains(out, "range 120 nm") {
			t.Fatalf("press %d: range pane rendered without a receiver position", i)
		}
	}
}

func TestThemeCycle(t *testing.T) {
	m := newModel(t)
	if m.theme.Name != "dark" {
		t.Fatalf("theme = %q, want dark", m.theme.Name)
	}
	m, _ = updCmd(t, m, key("d"))
	if m.theme.Name != "light" {
		t.Fatalf("after d: theme = %q, want light", m.theme.Name)
	}
	m, _ = updCmd(t, m, key("d"))
	if m.theme.Name != "dark" {
		t.Fatalf("after second d: theme = %q, want dark", m.theme.Name)
	}
}

func TestHelpToggle(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m, _ = updCmd(t, m, key("h"))
	if !m.help {
		t.Fatal("help not open")
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "cycle panes") || !strings.Contains(out, "keys") {
		t.Errorf("help view missing key reference:\n%s", out)
	}
	// esc closes help first...
	m, _ = updCmd(t, m, escKey)
	if m.help {
		t.Fatal("esc did not close help")
	}
	// ...and h toggles it both ways.
	m, _ = updCmd(t, m, key("h"))
	if !m.help {
		t.Fatal("second h did not open help")
	}
	m, _ = updCmd(t, m, key("h"))
	if m.help {
		t.Fatal("third h did not close help")
	}
}

func TestFilterFlow(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)
	if len(m.entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(m.entries))
	}

	// "/" opens the prompt with the previous query as the draft.
	m, _ = updCmd(t, m, key("/"))
	if !m.filtering {
		t.Fatal("/ did not open the filter")
	}
	if out := stripANSI(m.View()); !strings.Contains(out, "/_") {
		t.Errorf("header missing the live prompt: %s", out)
	}

	// Typing builds the draft; nothing is filtered yet.
	for _, ch := range "DLH" {
		next, cmd := updCmd(t, m, key(string(ch)))
		if cmd != nil {
			t.Fatal("typing produced a command")
		}
		m = next
	}
	if m.draft != "DLH" || m.filter != "" || len(m.entries) != 3 {
		t.Fatalf("draft=%q filter=%q entries=%d", m.draft, m.filter, len(m.entries))
	}

	// enter applies the filter: one aircraft survives.
	m, _ = updCmd(t, m, enterKey)
	if m.filtering || m.filter != "DLH" {
		t.Fatalf("filtering=%v filter=%q", m.filtering, m.filter)
	}
	if len(m.entries) != 1 || m.entries[0].Ac.Hex != "bbbbbb" {
		t.Fatalf("filtered entries = %v, want [bbbbbb]", hexOrder(m))
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, "filter DLH") {
		t.Errorf("header missing filter chip:\n%s", out)
	}
	if !strings.Contains(out, "aircraft 1/3") {
		t.Errorf("table title missing 1/3 counts:\n%s", out)
	}

	// esc clears the applied filter.
	m, _ = updCmd(t, m, escKey)
	if m.filter != "" || len(m.entries) != 3 {
		t.Fatalf("after esc: filter=%q entries=%d", m.filter, len(m.entries))
	}
}

func TestFilterSafetyKeys(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)

	// "q" inside the filter must type, not quit.
	m, _ = updCmd(t, m, key("/"))
	next, cmd := updCmd(t, m, key("q"))
	if cmd != nil {
		t.Fatal("q inside the filter produced a quit command")
	}
	if next.filtering != true || next.draft != "q" {
		t.Fatalf("draft = %q, want q", next.draft)
	}

	// Digits inside the filter must not switch sort modes.
	m = next
	next, _ = updCmd(t, m, key("1"))
	if next.sort != m.sort {
		t.Fatal("digit key changed the sort while filtering")
	}
	m = next // draft is now "q1"

	// backspace edits the draft.
	m, _ = updCmd(t, m, tea.KeyMsg{Type: tea.KeyBackspace})
	if m.draft != "q" {
		t.Fatalf("draft after backspace = %q, want q", m.draft)
	}

	// esc cancels the prompt without applying it.
	m, _ = updCmd(t, m, escKey)
	if m.filtering || m.filter != "" || len(m.entries) != 3 {
		t.Fatalf("cancel left filtering=%v filter=%q entries=%d", m.filtering, m.filter, len(m.entries))
	}
}

func TestFilterNoMatch(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)
	m, _ = updCmd(t, m, key("/"))
	for _, ch := range "ZZZ" {
		m, _ = updCmd(t, m, key(string(ch)))
	}
	m, _ = updCmd(t, m, enterKey)

	if len(m.entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(m.entries))
	}
	out := stripANSI(m.View())
	if !strings.Contains(out, `no aircraft match "ZZZ"`) {
		t.Errorf("missing empty-filter hint:\n%s", out)
	}
	if !strings.Contains(out, "0/3 ac") {
		t.Errorf("header missing 0/3 counts:\n%s", out)
	}
}

func TestFilterMatchesHex(t *testing.T) {
	m := newModel(t)
	m = withData(t, m)
	entries := Filter(m.entries, "cc")
	if len(entries) != 1 || entries[0].Ac.Hex != "cccccc" {
		t.Fatalf("hex filter = %v, want [cccccc]", entries)
	}
}

func TestSortKeys(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)

	tests := []struct {
		name  string
		key   string
		want  SortKey
		order []string
	}{
		{name: "2 sorts by callsign", key: "2", want: SortCallsign,
			order: []string{"bbbbbb", "cccccc", "aaaaaa"}},
		{name: "3 sorts by altitude", key: "3", want: SortAltitude,
			order: []string{"aaaaaa", "cccccc", "bbbbbb"}},
		{name: "1 sorts by distance", key: "1", want: SortDistance,
			order: []string{"aaaaaa", "bbbbbb", "cccccc"}},
	}
	// Default order before any key: nearest first.
	if got := hexOrder(m); strings.Join(got, ",") != "aaaaaa,bbbbbb,cccccc" {
		t.Fatalf("default order = %v, want distance order", got)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd tea.Cmd
			m, cmd = updCmd(t, m, key(tt.key))
			if cmd != nil {
				t.Error("sort key produced a command")
			}
			if m.sort != tt.want {
				t.Fatalf("sort = %v, want %v", m.sort, tt.want)
			}
			got := strings.Join(hexOrder(m), ",")
			if want := strings.Join(tt.order, ","); got != want {
				t.Fatalf("order = %v, want %v", got, want)
			}
			if out := stripANSI(m.View()); !strings.Contains(out, "sort "+tt.want.String()) {
				t.Errorf("header missing %q", "sort "+tt.want.String())
			}
		})
	}
}

func TestApplyAircraft(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	now := time.Now()

	// Success stores the fleet and marks the view as fetched.
	m2, cmd := updCmd(t, m, aircraftMsg{doc: testDoc(), at: now})
	if cmd == nil {
		t.Error("refresh not scheduled after a successful fetch")
	}
	if m2.err != "" || len(m2.entries) != 3 || !m2.fetched {
		t.Fatalf("err=%q entries=%d fetched=%v", m2.err, len(m2.entries), m2.fetched)
	}
	if !m2.updated.Equal(now) {
		t.Errorf("updated = %v, want %v", m2.updated, now)
	}

	// Failure keeps the previous data and surfaces the error.
	failing := aircraftMsg{err: errors.New("fetch failed"), at: now.Add(time.Second)}
	m3, cmd := updCmd(t, m2, failing)
	if cmd == nil {
		t.Error("refresh not scheduled after a failed fetch")
	}
	if !strings.Contains(m3.err, "fetch failed") {
		t.Fatalf("err = %q", m3.err)
	}
	if len(m3.entries) != 3 || !m3.fetched {
		t.Fatal("failed fetch dropped the previous data")
	}
	out := stripANSI(m3.View())
	if !strings.Contains(out, "err: fetch failed") {
		t.Errorf("footer missing fetch error:\n%s", out)
	}

	// A later success clears the error.
	m4, _ := updCmd(t, m3, aircraftMsg{doc: testDoc(), at: now.Add(2 * time.Second)})
	if m4.err != "" {
		t.Errorf("err = %q after recovery, want empty", m4.err)
	}
}

func TestApplyAircraftPausedDoesNotReschedule(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m, _ = updCmd(t, m, key("p"))
	if !m.paused {
		t.Fatal("model not paused")
	}
	m2, cmd := updCmd(t, m, aircraftMsg{doc: testDoc(), at: time.Now()})
	if cmd != nil {
		t.Error("paused model scheduled a refresh")
	}
	if len(m2.entries) != 3 {
		t.Errorf("entries = %d, want 3 (data still applies)", len(m2.entries))
	}
}

func TestKeyNavigation(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)

	steps := []struct {
		name string
		msg  tea.KeyMsg
		want int
	}{
		{name: "j down", msg: key("j"), want: 1},
		{name: "k up", msg: key("k"), want: 0},
		{name: "up arrow", msg: tea.KeyMsg{Type: tea.KeyUp}, want: 0}, // clamped at 0
		{name: "down arrow", msg: tea.KeyMsg{Type: tea.KeyDown}, want: 1},
		{name: "end", msg: tea.KeyMsg{Type: tea.KeyEnd}, want: 2},
		{name: "down clamps at last", msg: key("j"), want: 2},
		{name: "home", msg: tea.KeyMsg{Type: tea.KeyHome}, want: 0},
		{name: "pgdn", msg: tea.KeyMsg{Type: tea.KeyPgDown}, want: 2},
		{name: "pgup", msg: tea.KeyMsg{Type: tea.KeyPgUp}, want: 0},
	}
	for _, tt := range steps {
		t.Run(tt.name, func(t *testing.T) {
			var cmd tea.Cmd
			m, cmd = updCmd(t, m, tt.msg)
			if cmd != nil {
				t.Error("navigation produced a command")
			}
			if m.cursor != tt.want {
				t.Fatalf("cursor = %d, want %d", m.cursor, tt.want)
			}
		})
	}

	// An empty document must reset the cursor instead of dangling.
	m, _ = updCmd(t, m, aircraftMsg{doc: client.Doc{}, at: time.Now()})
	if m.cursor != 0 || len(m.entries) != 0 {
		t.Fatalf("cursor=%d entries=%d after empty doc", m.cursor, len(m.entries))
	}
}

func TestViewBeforeSizeIsBlank(t *testing.T) {
	if out := newModel(t).View(); out != "" {
		t.Fatalf("View before WindowSizeMsg = %q, want empty", out)
	}
}

func TestView80x24(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	m = withData(t, m)
	lines := checkFrame(t, m.View(), 80, 24)

	full := strings.Join(lines, "\n")
	for _, want := range []string{
		"adsb-tui", "3 ac", "view split", "sort distance",
		"range 120 nm", "aircraft 3", "CALLSIGN",
		"q quit", "1/2/3 sort",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("view missing %q", want)
		}
	}

	// Left 36 columns are the range pane, right 43 the table (one
	// gutter column between them).
	var rangePart, tablePart strings.Builder
	for _, l := range lines[1:23] {
		rangePart.WriteString(l[:36])
		tablePart.WriteString(l[37:])
	}
	if !strings.Contains(rangePart.String(), "UAL123 350") {
		t.Errorf("range pane missing the callsign label:\n%s", rangePart.String())
	}
	if !strings.Contains(rangePart.String(), "@") {
		t.Error("range pane missing the selected blip")
	}
	if !strings.Contains(tablePart.String(), "UAL123") || !strings.Contains(tablePart.String(), "aaaaaa") {
		t.Errorf("table missing fixture rows:\n%s", tablePart.String())
	}
}

func TestView200x50(t *testing.T) {
	m := sized(t, newModel(t), 200, 50)
	m = withData(t, m)
	lines := checkFrame(t, m.View(), 200, 50)

	full := strings.Join(lines, "\n")
	for _, want := range []string{"adsb-tui", "view split", "range 120 nm", "aircraft 3", "UAL123"} {
		if !strings.Contains(full, want) {
			t.Errorf("view missing %q", want)
		}
	}
	var rangePart strings.Builder
	for _, l := range lines[1:49] {
		rangePart.WriteString(l[:60])
	}
	if !strings.Contains(rangePart.String(), "UAL123 350") {
		t.Errorf("range pane missing the callsign label:\n%s", rangePart.String())
	}
}

func TestViewSmallHidesLabels(t *testing.T) {
	// 46x14 stacks a 6-row range pane over the table: short enough that
	// only marker blips fit, no callsign labels.
	m := sized(t, newModel(t), 46, 14)
	m = withData(t, m)
	lines := checkFrame(t, m.View(), 46, 14)

	rangeSection := strings.Join(lines[1:7], "\n")
	tableSection := strings.Join(lines[7:13], "\n")

	if !strings.Contains(rangeSection, "@") && !strings.Contains(rangeSection, "*") {
		t.Errorf("range pane lost its blips:\n%s", rangeSection)
	}
	for _, hidden := range []string{"UAL123", "DLH441", "SWR13F"} {
		if strings.Contains(rangeSection, hidden) {
			t.Errorf("range pane still shows label %q:\n%s", hidden, rangeSection)
		}
	}
	// The table right below still lists the aircraft.
	if !strings.Contains(tableSection, "UAL123") {
		t.Errorf("table section missing callsign:\n%s", tableSection)
	}
}

func TestViewReceiverMissingWarning(t *testing.T) {
	m := New(Options{
		Source:       &stubSource{doc: testDoc()},
		HasReceiver:  false,
		ReceiverWarn: "receiver position unknown - table only",
		RangeNm:      120,
	})
	m = sized(t, m, 80, 24)
	m = withData(t, m)
	out := stripANSI(m.View())
	checkFrame(t, m.View(), 80, 24)

	if !strings.Contains(out, "receiver position unknown - table only") {
		t.Errorf("footer missing the receiver warning:\n%s", out)
	}
	if strings.Contains(out, "range 120 nm") {
		t.Errorf("polar plot rendered without a receiver position:\n%s", out)
	}
	if !strings.Contains(out, "view table") {
		t.Errorf("header missing view table:\n%s", out)
	}

	// The pane key must not resurrect the disabled plot.
	m, _ = updCmd(t, m, key("t"))
	out = stripANSI(m.View())
	if strings.Contains(out, "range 120 nm") || !strings.Contains(out, "view table") {
		t.Errorf("t key enabled the plot without a receiver:\n%s", out)
	}
	if !strings.Contains(out, "receiver position unknown - table only") {
		t.Error("warning disappeared after pane toggle")
	}
}

func TestViewEmergencyBanner(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	doc := client.Doc{Aircraft: []client.Aircraft{
		{Hex: "ff0001", Flight: "RESCUE1 ", Lat: fp(48.21), Lon: fp(11.51),
			AltBaro: json.RawMessage(`"ground"`), Squawk: "7700"},
	}}
	m = upd(t, m, aircraftMsg{doc: doc, at: time.Now()})
	out := stripANSI(m.View())

	if !strings.Contains(out, "emergency x1") {
		t.Errorf("header missing emergency banner:\n%s", out)
	}
	if !strings.Contains(out, "GND") {
		t.Errorf("ground aircraft not rendered as GND:\n%s", out)
	}
}

func TestViewEmptyHints(t *testing.T) {
	m := sized(t, newModel(t), 80, 24)
	if out := stripANSI(m.View()); !strings.Contains(out, "waiting for data...") {
		t.Errorf("missing initial hint:\n%s", out)
	}
	m = upd(t, m, aircraftMsg{doc: client.Doc{}, at: time.Now()})
	if out := stripANSI(m.View()); !strings.Contains(out, "no aircraft") {
		t.Errorf("missing empty-fleet hint:\n%s", out)
	}
}

func TestViewDegenerateSizes(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		want int // expected line count; -1 for an empty view
	}{
		{name: "zero", w: 0, h: 0, want: -1},
		{name: "one cell", w: 1, h: 1, want: 1},
		{name: "two rows lose the content area", w: 10, h: 2, want: 2},
		{name: "five by five", w: 5, h: 5, want: 5},
		{name: "narrow tall", w: 2, h: 10, want: 10},
		{name: "wide flat", w: 120, h: 3, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sized(t, newModel(t), tt.w, tt.h)
			m = withData(t, m)
			out := m.View()
			if tt.want == -1 {
				if out != "" {
					t.Fatalf("View = %q, want empty", out)
				}
				return
			}
			checkFrame(t, out, tt.w, tt.want)
		})
	}
}

func TestFetchCmd(t *testing.T) {
	t.Run("stub source answers", func(t *testing.T) {
		src := &stubSource{doc: testDoc()}
		m := New(Options{Source: src})
		msg := m.fetchCmd()()
		am, ok := msg.(aircraftMsg)
		if !ok {
			t.Fatalf("message type = %T, want aircraftMsg", msg)
		}
		if am.err != nil || len(am.doc.Aircraft) != 3 {
			t.Fatalf("err=%v aircraft=%d", am.err, len(am.doc.Aircraft))
		}
		if src.calls != 1 {
			t.Errorf("source calls = %d, want 1", src.calls)
		}
	})
	t.Run("nil source reports an error", func(t *testing.T) {
		m := New(Options{})
		msg := m.fetchCmd()()
		am := msg.(aircraftMsg)
		if am.err == nil || !strings.Contains(am.err.Error(), "no data source") {
			t.Fatalf("err = %v, want no data source", am.err)
		}
	})
	t.Run("source errors propagate", func(t *testing.T) {
		src := &stubSource{err: errors.New("decoder down")}
		m := New(Options{Source: src})
		am := m.fetchCmd()().(aircraftMsg)
		if am.err == nil || !strings.Contains(am.err.Error(), "decoder down") {
			t.Fatalf("err = %v, want decoder down", am.err)
		}
	})
}
