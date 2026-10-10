package render

import (
	"reflect"
	"testing"
)

func TestByName(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "", want: "dark"},
		{name: "dark", want: "dark"},
		{name: "light", want: "light"},
		{name: "DARK", wantErr: true},
		{name: "sepia", wantErr: true},
	}
	for _, tt := range tests {
		t.Run("name="+tt.name, func(t *testing.T) {
			th, err := ByName(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ByName(%q) succeeded, want an error", tt.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("ByName(%q): %v", tt.name, err)
			}
			if th.Name != tt.want {
				t.Fatalf("ByName(%q).Name = %q, want %q", tt.name, th.Name, tt.want)
			}
		})
	}
}

func TestNextThemeCycles(t *testing.T) {
	tests := []struct {
		name string
		in   Theme
		want string
	}{
		{name: "dark to light", in: Dark(), want: "light"},
		{name: "light to dark", in: Light(), want: "dark"},
		{name: "zero theme goes light", in: Theme{}, want: "light"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Next(tt.in); got.Name != tt.want {
				t.Fatalf("Next().Name = %q, want %q", got.Name, tt.want)
			}
		})
	}
	// Two cycles return to the starting theme.
	back := Next(Next(Dark()))
	if back.Name != "dark" {
		t.Fatalf("two cycles landed on %q, want dark", back.Name)
	}
}

func TestThemeStylesDiffer(t *testing.T) {
	d, l := Dark(), Light()
	if d.Name == l.Name {
		t.Fatal("themes share a name")
	}
	for _, a := range []Attr{AttrBorder, AttrBlip, AttrWarn, AttrAccent, AttrSel} {
		if reflect.DeepEqual(d.styles[a], l.styles[a]) {
			t.Errorf("attr %v is styled identically in both themes", a)
		}
	}
	// Every attribute the renderer uses must be styled in both themes.
	for _, a := range []Attr{AttrBorder, AttrTitle, AttrHeader, AttrRing, AttrSweep,
		AttrBlip, AttrLabel, AttrWarn, AttrSel, AttrDim, AttrAccent} {
		if _, ok := d.styles[a]; !ok {
			t.Errorf("dark theme missing attr %v", a)
		}
		if _, ok := l.styles[a]; !ok {
			t.Errorf("light theme missing attr %v", a)
		}
	}
}

func TestZeroThemeRendersPlain(t *testing.T) {
	var zero Theme
	// No registered styles: rendering must still produce the text.
	if got := zero.Style(AttrBlip).Render("x"); got != "x" {
		t.Fatalf("zero theme render = %q, want %q", got, "x")
	}
}
