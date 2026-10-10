package render

import "strings"

// Box frames content in an ASCII box of exactly w x h cells: a top
// border carrying the title, a bottom border, and one '|' on each side.
// Content lines are clipped or padded to the inner width; missing lines
// are blank. Degenerate sizes never panic: a box that cannot be drawn
// degrades to a fragment or nothing at all.
func Box(content []string, w, h int, title string, th Theme) []string {
	if w <= 0 || h <= 0 {
		return nil
	}
	inner := w - 2
	if inner < 0 {
		inner = 0
	}
	border := th.Style(AttrBorder)

	var out []string
	if h >= 1 {
		out = append(out, boxTop(inner, title, th))
	}
	// Degenerate single-row box: the top border is the whole box.
	if h == 1 {
		return out
	}

	body := h - 2
	left, right := border.Render("|"), border.Render("|")
	for i := 0; i < body; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		out = append(out, left+PadRight(line, inner)+right)
	}
	out = append(out, border.Render("+"+strings.Repeat("-", inner)+"+"))
	return out
}

// boxTop builds the top border, weaving the title into the dashes when
// it fits and falling back to a plain border when it does not.
func boxTop(inner int, title string, th Theme) string {
	border := th.Style(AttrBorder)
	if inner <= 0 {
		return border.Render("++")
	}
	dashes := strings.Repeat("-", inner)
	label := ""
	if title != "" {
		label = " " + fitTitle(title, inner-2) + " "
		if ansiWidth(label) > inner {
			label = ""
		}
	}
	if label == "" {
		return border.Render("+" + dashes + "+")
	}
	// One dash before the label, the rest after it, so the title reads
	// as "+- title -----+".
	pad := 1
	rest := inner - pad - ansiWidth(label)
	if rest < 0 {
		return border.Render("+" + dashes + "+")
	}
	return border.Render("+") +
		border.Render(strings.Repeat("-", pad)) +
		th.Style(AttrTitle).Render(label) +
		border.Render(strings.Repeat("-", rest)+"+")
}

// fitTitle clips a title to w cells so it can never widen the border.
func fitTitle(title string, w int) string {
	if w <= 0 {
		return ""
	}
	return Fit(title, w)
}
