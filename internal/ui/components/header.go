package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/ui/style"
	"github.com/sadopc/godu/internal/util"
)

// RenderHeader renders the top header bar.
func RenderHeader(theme style.Theme, root *model.DirNode, width int) string {
	if root == nil || width < 10 {
		return ""
	}

	titleStr := " godu"
	titleStyled := lipgloss.NewStyle().Bold(true).Foreground(theme.Primary).Render(titleStr)

	stats := fmt.Sprintf("%s items  %s ",
		util.FormatCount(root.ItemCount),
		util.FormatSize(root.GetSize()),
	)
	statsStyled := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(stats)

	titleW := lipgloss.Width(titleStyled)
	statsW := lipgloss.Width(statsStyled)

	// Path gets whatever space remains
	pathMaxW := width - titleW - statsW - 3 // 3 for "  " separator + safety
	pathStr := root.Name
	if pathMaxW > 5 {
		pathStr = util.TruncateString(pathStr, pathMaxW)
	} else {
		pathStr = ""
	}

	pathStyled := lipgloss.NewStyle().Foreground(theme.TextPrimary).Render("  " + pathStr)
	pathW := lipgloss.Width(pathStyled)

	gap := width - titleW - pathW - statsW
	if gap < 1 {
		gap = 1
	}

	line := titleStyled + pathStyled + strings.Repeat(" ", gap) + statsStyled
	return theme.HeaderStyle.Width(width).Render(line)
}

// RenderBreadcrumb renders the breadcrumb path navigation.
func RenderBreadcrumb(theme style.Theme, current *model.DirNode, width int) string {
	if current == nil {
		return ""
	}

	// Collect path segments (skip root which is already in the header)
	var segments []string
	node := current
	for node != nil {
		if node.Parent == nil {
			// Root — show a "/" or the base dir name
			segments = append([]string{"/"}, segments...)
		} else {
			segments = append([]string{node.Name}, segments...)
		}
		node = node.Parent
	}

	sep := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(" > ")
	var parts []string
	for i, seg := range segments {
		s := lipgloss.NewStyle().Foreground(theme.TextMuted)
		if i == len(segments)-1 {
			s = lipgloss.NewStyle().Foreground(theme.TextPrimary).Bold(true)
		}
		parts = append(parts, s.Render(seg))
	}

	breadcrumb := " " + strings.Join(parts, sep)

	// Truncate if too wide
	if lipgloss.Width(breadcrumb) > width {
		// Show just the last 2 segments
		if len(parts) > 2 {
			ellipsis := lipgloss.NewStyle().Foreground(theme.TextMuted).Render("...")
			breadcrumb = " " + ellipsis + sep + strings.Join(parts[len(parts)-2:], sep)
		}
	}

	return theme.BreadcrumbStyle.Width(width).Render(breadcrumb)
}
