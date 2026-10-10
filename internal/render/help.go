package render

// HelpKey is one entry of the key reference: the key or key
// combination and what it does.
type HelpKey struct {
	// Key is the literal key to press, ASCII only.
	Key string
	// Desc is the action the key performs.
	Desc string
}

// helpKeys is the built-in key reference, in display order.
var helpKeys = []HelpKey{
	{"q", "quit"},
	{"t", "cycle panes: split / map / table"},
	{"/", "filter by callsign"},
	{"enter", "apply the typed filter"},
	{"esc", "clear filter / close help"},
	{"h", "this help"},
	{"p", "pause updates"},
	{"d", "dark / light theme"},
	{"1", "sort by distance"},
	{"2", "sort by callsign"},
	{"3", "sort by altitude"},
	{"up/down", "move the selection"},
	{"pgup/pgdn", "scroll the table"},
}

// HelpKeys returns a copy of the key reference in display order.
func HelpKeys() []HelpKey {
	out := make([]HelpKey, len(helpKeys))
	copy(out, helpKeys)
	return out
}

// Help renders the key reference into exactly h lines of w cells:
// "key" left-aligned in a fixed column, then the description. Lines
// that do not fit are dropped and missing lines left blank, so the
// result always matches the requested height.
func Help(w, h int, th Theme) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	keyCol := 9
	keys := th.Style(AttrAccent)
	desc := th.Style(AttrDim)

	out := make([]string, 0, h)
	for i := 0; i < h; i++ {
		if i >= len(helpKeys) {
			out = append(out, "")
			continue
		}
		k := helpKeys[i]
		line := keys.Render(PadRight(k.Key, keyCol)) + desc.Render(k.Desc)
		out = append(out, Fit(line, w))
	}
	return out
}
