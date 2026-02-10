package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layout manages the arrangement of UI components within terminal dimensions.
type Layout struct {
	Width  int
	Height int
}

// NewLayout creates a layout for the given terminal dimensions.
func NewLayout(width, height int) Layout {
	return Layout{Width: width, Height: height}
}

// ContentHeight returns the height available for the main content area.
func (l Layout) ContentHeight() int {
	h := l.Height - 4 // header + breadcrumb + tabbar + statusbar
	if h < 1 {
		h = 1
	}
	return h
}

// ContentWidth returns the width available for the main content area.
func (l Layout) ContentWidth() int {
	if l.Width < 20 {
		return 20
	}
	return l.Width
}

// BarWidth returns the width for progress bars in tree view.
func (l Layout) BarWidth() int {
	bar := l.ContentWidth() - l.rowOverhead()
	if bar < 5 {
		bar = 5
	}
	if bar > 40 {
		bar = 40
	}
	return bar
}

// NameWidth returns the width available for file/dir names.
func (l Layout) NameWidth() int {
	w := l.ContentWidth() - l.rowOverhead() - l.BarWidth()
	if w < 8 {
		w = 8
	}
	return w
}

// rowOverhead returns the fixed-width portion of each tree view row
// (everything except the bar and name).
//
// Layout: "  " mark + "99.9%" pct(6) + " [" + bar + "] " + name + " " + "  9.9 GiB" size(10)
// Fixed:    2         + 6             + 2    +     + 2    +      + 1  + 10 = 23
func (l Layout) rowOverhead() int {
	return 23 // mark(2) + pct(6) + " ["(2) + "] "(2) + " "(1) + size(10)
}

// Center centers content in the available width.
func (l Layout) Center(content string) string {
	return lipgloss.PlaceHorizontal(l.Width, lipgloss.Center, content)
}

// FullWidth pads a string with spaces to reach exactly the target visual width.
// If the string is already wider, it is returned as-is (no truncation).
func FullWidth(s string, width int) string {
	visLen := lipgloss.Width(s)
	if visLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visLen)
}
