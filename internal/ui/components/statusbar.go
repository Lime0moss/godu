package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/ui/style"
	"github.com/sadopc/godu/internal/util"
)

// StatusInfo holds the current state for the status bar.
type StatusInfo struct {
	CurrentDir  *model.DirNode
	MarkedCount int
	MarkedSize  int64
	UseApparent bool
	ShowHidden  bool
	SortField   model.SortField
	ViewMode    int
	ErrorMsg    string
}

// RenderStatusBar renders the bottom status bar.
func RenderStatusBar(theme style.Theme, info StatusInfo, width int) string {
	if info.ErrorMsg != "" {
		errLine := " " + lipgloss.NewStyle().Foreground(theme.Warning).Bold(true).Render(info.ErrorMsg)
		return theme.StatusBarStyle.Width(width).Render(errLine)
	}

	var parts []string

	if info.CurrentDir != nil {
		count := len(info.CurrentDir.GetChildren())
		parts = append(parts, fmt.Sprintf("%d items", count))

		var size int64
		if info.UseApparent {
			size = info.CurrentDir.GetSize()
		} else {
			size = info.CurrentDir.GetUsage()
		}
		sizeLabel := "disk"
		if info.UseApparent {
			sizeLabel = "apparent"
		}
		parts = append(parts, fmt.Sprintf("%s %s", util.FormatSize(size), sizeLabel))
	}

	if info.MarkedCount > 0 {
		marked := lipgloss.NewStyle().
			Foreground(theme.Error).
			Bold(true).
			Render(fmt.Sprintf("* %d marked (%s)", info.MarkedCount, util.FormatSize(info.MarkedSize)))
		parts = append(parts, marked)
	}

	left := " " + strings.Join(parts, " | ")

	hints := []struct{ key, desc string }{
		{"?", "help"},
		{"d", "delete"},
		{"q", "quit"},
	}

	var rightParts []string
	for _, h := range hints {
		k := lipgloss.NewStyle().Foreground(theme.Primary).Bold(true).Render(h.key)
		d := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(" " + h.desc)
		rightParts = append(rightParts, k+d)
	}
	right := strings.Join(rightParts, "  ") + " "

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + right
	return theme.StatusBarStyle.Width(width).Render(line)
}

// RenderTabBar renders the view mode tab bar.
func RenderTabBar(theme style.Theme, activeView int, sortField model.SortField, width int) string {
	tabs := []string{"Tree View", "Treemap", "File Types"}

	var tabLine []string
	for i, tab := range tabs {
		label := fmt.Sprintf(" %d %s ", i+1, tab)
		if i == activeView {
			tabLine = append(tabLine, theme.TabActiveStyle.Render(label))
		} else {
			tabLine = append(tabLine, theme.TabInactiveStyle.Render(label))
		}
	}

	left := " " + strings.Join(tabLine, " ")

	sortNames := map[model.SortField]string{
		model.SortBySize:  "Size",
		model.SortByName:  "Name",
		model.SortByCount: "Count",
		model.SortByMtime: "Mtime",
	}

	sortLabel := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Render("Sort: " + sortNames[sortField] + " ")

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(sortLabel)
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	line := left + strings.Repeat(" ", gap) + sortLabel
	return lipgloss.NewStyle().
		Foreground(theme.TextSecondary).
		Background(theme.BgLight).
		Width(width).
		Render(line)
}
