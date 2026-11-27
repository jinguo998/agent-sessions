package shared

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/jinguo998/agent-sessions/internal/domain"
)

// SourcePalette holds terminal 256-color codes for light and dark backgrounds.
type SourcePalette struct {
	Light string
	Dark  string
}

var sourcePalettes = map[domain.Source]SourcePalette{
	domain.SourceClaude:         {Light: "27", Dark: "75"},
	domain.SourceCodex:          {Light: "22", Dark: "40"},
	domain.SourceOpenCode:       {Light: "88", Dark: "204"},
	domain.SourceAntigravityIDE: {Light: "54", Dark: "141"},
	domain.SourceAntigravityCLI: {Light: "30", Dark: "80"},
}

var fallbackSourcePalette = SourcePalette{Light: "242", Dark: "241"}

// Adaptive returns a Lip Gloss color adapted for light and dark terminals.
func Adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// SourceColors returns the terminal color pair for a session source.
func SourceColors(src domain.Source) SourcePalette {
	if palette, ok := sourcePalettes[src]; ok {
		return palette
	}
	return fallbackSourcePalette
}

// SourceBadgeStyle returns the Lip Gloss style used to render a source badge.
func SourceBadgeStyle(src domain.Source) lipgloss.Style {
	palette := SourceColors(src)
	return lipgloss.NewStyle().
		Foreground(Adaptive(palette.Light, palette.Dark)).
		Bold(true)
}

// SourceANSIColor returns a bold ANSI 256-color foreground sequence matching the
// current terminal background polarity (without emitting a full SGR reset).
func SourceANSIColor(src domain.Source) string {
	palette := SourceColors(src)
	color := palette.Light
	if lipgloss.HasDarkBackground() {
		color = palette.Dark
	}
	return fmt.Sprintf("\033[1;38;5;%sm", color)
}
