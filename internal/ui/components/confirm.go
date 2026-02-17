package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/serdar/godu/internal/ui/style"
	"github.com/serdar/godu/internal/util"
)

// ConfirmItem represents an item pending deletion.
type ConfirmItem struct {
	Name  string
	Path  string // Full path used for deletion
	Size  int64
	IsDir bool
}

// RenderConfirmDialog renders the deletion confirmation modal.
func RenderConfirmDialog(theme style.Theme, items []ConfirmItem, width, height int) string {
	boxWidth := 60
	if boxWidth > width-4 {
		boxWidth = width - 4
	}

	var lines []string

	title := theme.ModalTitle.Render("  Delete Confirmation")
	lines = append(lines, title)

	warning := lipgloss.NewStyle().
		Foreground(theme.Warning).
		Render(fmt.Sprintf("  The following %d item(s) will be permanently deleted:", len(items)))
	lines = append(lines, warning)
	lines = append(lines, "")

	maxShow := 10
	if len(items) < maxShow {
		maxShow = len(items)
	}

	var totalSize int64
	for _, item := range items {
		totalSize += item.Size
	}

	for i := 0; i < maxShow; i++ {
		item := items[i]
		icon := "  F "
		if item.IsDir {
			icon = "  D "
		}
		name := util.TruncateString(item.Name, boxWidth-20)
		size := util.FormatSize(item.Size)
		line := lipgloss.NewStyle().Foreground(theme.Error).Render(icon+name) +
			lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  "+size)
		lines = append(lines, line)
	}

	if len(items) > maxShow {
		more := fmt.Sprintf("  ... and %d more", len(items)-maxShow)
		lines = append(lines, lipgloss.NewStyle().Foreground(theme.TextMuted).Render(more))
	}

	lines = append(lines, "")
	totalLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.TextPrimary).
		Render(fmt.Sprintf("  Total: %s", util.FormatSize(totalSize)))
	lines = append(lines, totalLine)
	lines = append(lines, "")

	prompt := lipgloss.NewStyle().
		Foreground(theme.TextPrimary).
		Render("  Press ") +
		lipgloss.NewStyle().Bold(true).Foreground(theme.Success).Render("y") +
		lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(" to confirm, ") +
		lipgloss.NewStyle().Bold(true).Foreground(theme.Error).Render("n/esc") +
		lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(" to cancel")
	lines = append(lines, prompt)

	content := strings.Join(lines, "\n")

	box := theme.ModalStyle.
		Width(boxWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
