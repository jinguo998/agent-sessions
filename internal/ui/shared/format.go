package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

// RelativeTime formats t as a compact relative duration string.
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// DisplayWidth returns the terminal column width of s, counting wide CJK glyphs as 2.
func DisplayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// TruncateToWidth truncates s with an ellipsis so it fits within maxWidth display columns.
func TruncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}

	budget := maxWidth - 3
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if w+rw > budget {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "..."
}

// PadToWidth right-pads s with spaces to reach targetWidth display columns.
func PadToWidth(s string, targetWidth int) string {
	w := DisplayWidth(s)
	if w >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-w)
}

// WrapText wraps text to the given display column width. If width <= 0, returns text unchanged.
func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if DisplayWidth(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}
		runes := []rune(line)
		w := 0
		start := 0
		for i, r := range runes {
			rw := runewidth.RuneWidth(r)
			if w+rw > width {
				result.WriteString(string(runes[start:i]))
				result.WriteString("\n")
				start = i
				w = 0
			}
			w += rw
		}
		if start < len(runes) {
			result.WriteString(string(runes[start:]))
			result.WriteString("\n")
		}
	}
	return strings.TrimRight(result.String(), "\n")
}
