package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Theme holds all the styled components for the UI.
type Theme struct {
	// Base colors
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Muted      lipgloss.Color
	Error      lipgloss.Color
	Warning    lipgloss.Color
	Success    lipgloss.Color

	// Backgrounds
	BgDark     lipgloss.Color
	BgMedium   lipgloss.Color
	BgLight    lipgloss.Color
	BgSelected lipgloss.Color

	// Text
	TextPrimary   lipgloss.Color
	TextSecondary lipgloss.Color
	TextMuted     lipgloss.Color

	// Gradient colors for bars
	GradientStart lipgloss.Color
	GradientEnd   lipgloss.Color

	// Styles
	HeaderStyle      lipgloss.Style
	BreadcrumbStyle  lipgloss.Style
	TabActiveStyle   lipgloss.Style
	TabInactiveStyle lipgloss.Style
	StatusBarStyle   lipgloss.Style
	SelectedRow      lipgloss.Style
	NormalRow        lipgloss.Style
	MarkedIndicator  lipgloss.Style
	CursorIndicator  lipgloss.Style
	DirName          lipgloss.Style
	FileName         lipgloss.Style
	SizeText         lipgloss.Style
	PercentText      lipgloss.Style
	ErrorText        lipgloss.Style
	HelpKey          lipgloss.Style
	HelpDesc         lipgloss.Style
	ModalStyle       lipgloss.Style
	ModalTitle       lipgloss.Style
	BorderStyle      lipgloss.Style
}

// DefaultTheme returns the default dark theme.
func DefaultTheme() Theme {
	t := Theme{
		Primary:   lipgloss.Color("#7B2FBE"),
		Secondary: lipgloss.Color("#00D4AA"),
		Accent:    lipgloss.Color("#61AFEF"),
		Muted:     lipgloss.Color("#5C6370"),
		Error:     lipgloss.Color("#E06C75"),
		Warning:   lipgloss.Color("#E5C07B"),
		Success:   lipgloss.Color("#98C379"),

		BgDark:     lipgloss.Color("#1E1E2E"),
		BgMedium:   lipgloss.Color("#282A36"),
		BgLight:    lipgloss.Color("#313244"),
		BgSelected: lipgloss.Color("#3E4451"),

		TextPrimary:   lipgloss.Color("#CDD6F4"),
		TextSecondary: lipgloss.Color("#BAC2DE"),
		TextMuted:     lipgloss.Color("#6C7086"),

		GradientStart: lipgloss.Color("#7B2FBE"),
		GradientEnd:   lipgloss.Color("#00D4AA"),
	}

	// Header: no padding — we handle spacing manually inside
	t.HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.TextPrimary).
		Background(t.BgMedium)

	t.BreadcrumbStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted)

	t.TabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.TextPrimary).
		Background(t.Primary).
		Padding(0, 1)

	t.TabInactiveStyle = lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Padding(0, 1)

	t.StatusBarStyle = lipgloss.NewStyle().
		Foreground(t.TextSecondary).
		Background(t.BgMedium)

	t.SelectedRow = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4A4A6A"))

	t.NormalRow = lipgloss.NewStyle().
		Foreground(t.TextSecondary)

	t.MarkedIndicator = lipgloss.NewStyle().
		Foreground(t.Error).
		Bold(true)

	t.CursorIndicator = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	t.DirName = lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	t.FileName = lipgloss.NewStyle().
		Foreground(t.TextSecondary)

	t.SizeText = lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Align(lipgloss.Right)

	t.PercentText = lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(6).
		Align(lipgloss.Right)

	t.ErrorText = lipgloss.NewStyle().
		Foreground(t.Error)

	t.HelpKey = lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	t.HelpDesc = lipgloss.NewStyle().
		Foreground(t.TextMuted)

	t.ModalStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Background(t.BgMedium)

	t.ModalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(t.TextPrimary).
		Padding(0, 0, 1, 0)

	t.BorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Muted)

	return t
}

// GradientColor returns a color interpolated between gradient start and end.
func (t Theme) GradientColor(ratio float64) lipgloss.Color {
	if ratio <= 0 {
		return t.GradientStart
	}
	if ratio >= 1 {
		return t.GradientEnd
	}

	c1, _ := colorful.Hex(string(t.GradientStart))
	c2, _ := colorful.Hex(string(t.GradientEnd))
	blended := c1.BlendLab(c2, ratio)
	return lipgloss.Color(blended.Hex())
}

// BarGradient renders a per-character gradient progress bar.
// Each filled character gets a unique color interpolated across the gradient.
func (t Theme) BarGradient(width int, ratio float64) string {
	if width <= 0 {
		return ""
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}

	var buf strings.Builder
	buf.Grow(width * 20) // rough estimate with ANSI codes

	c1, _ := colorful.Hex(string(t.GradientStart))
	c2, _ := colorful.Hex(string(t.GradientEnd))

	for i := 0; i < filled; i++ {
		// Each character gets its own gradient position
		charRatio := float64(i) / float64(max(width-1, 1))
		blended := c1.BlendLab(c2, charRatio)
		color := lipgloss.Color(blended.Hex())
		buf.WriteString(lipgloss.NewStyle().Foreground(color).Render("━"))
	}

	if filled < width {
		dimStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		buf.WriteString(dimStyle.Render(strings.Repeat("─", width-filled)))
	}

	return buf.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
