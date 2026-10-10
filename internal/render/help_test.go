package render

import (
	"strings"
	"testing"
)

func TestHelpRender(t *testing.T) {
	t.Run("exact height", func(t *testing.T) {
		lines := plainLines(Help(40, 16, Dark()))
		if len(lines) != 16 {
			t.Fatalf("Help returned %d lines, want 16", len(lines))
		}
		joined := strings.Join(lines, "\n")
		for _, want := range []string{"quit", "filter by callsign", "pause updates", "sort by altitude", "pgup/pgdn"} {
			if !strings.Contains(joined, want) {
				t.Errorf("help missing %q", want)
			}
		}
	})
	t.Run("clipped to requested height", func(t *testing.T) {
		lines := plainLines(Help(40, 3, Dark()))
		if len(lines) != 3 {
			t.Fatalf("Help returned %d lines, want 3", len(lines))
		}
		if !strings.Contains(lines[0], "quit") {
			t.Errorf("first line = %q, want the q entry", lines[0])
		}
	})
	t.Run("keys align in a fixed column", func(t *testing.T) {
		lines := plainLines(Help(40, len(HelpKeys()), Dark()))
		for i, k := range HelpKeys() {
			if !strings.HasPrefix(lines[i], k.Key) {
				t.Errorf("line %d = %q, want prefix %q", i, lines[i], k.Key)
			}
		}
	})
	t.Run("degenerate sizes", func(t *testing.T) {
		if got := Help(40, 0, Dark()); got != nil {
			t.Errorf("Help(h=0) = %v, want nil", got)
		}
		if got := Help(0, 5, Dark()); got != nil {
			t.Errorf("Help(w=0) = %v, want nil", got)
		}
	})
}

func TestHelpKeysCopyIsIndependent(t *testing.T) {
	keys := HelpKeys()
	if len(keys) == 0 {
		t.Fatal("HelpKeys returned nothing")
	}
	orig := keys[0].Key
	keys[0].Key = "mutated"
	if again := HelpKeys(); again[0].Key != orig {
		t.Fatalf("HelpKeys leaks mutations: got %q, want %q", again[0].Key, orig)
	}
}
