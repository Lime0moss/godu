package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/serdar/godu/internal/ui/style"
)

// RenderHelp renders the help overlay.
func RenderHelp(theme style.Theme, width, height int) string {
	boxWidth := 60
	if boxWidth > width-4 {
		boxWidth = width - 4
	}

	title := theme.ModalTitle.Render("  godu - Keyboard Shortcuts")

	sections := []struct {
		name  string
		binds []struct{ key, desc string }
	}{
		{
			name: "Navigation",
			binds: []struct{ key, desc string }{
				{"j/k", "Move up/down"},
				{"h/l", "Go to parent / enter directory"},
				{"Enter", "Enter directory"},
				{"Backspace", "Go back"},
			},
		},
		{
			name: "Views",
			binds: []struct{ key, desc string }{
				{"1", "Tree view"},
				{"2", "Treemap view"},
				{"3", "File type breakdown"},
			},
		},
		{
			name: "Sorting",
			binds: []struct{ key, desc string }{
				{"s", "Sort by size"},
				{"n", "Sort by name"},
				{"C", "Sort by item count"},
				{"M", "Sort by modification time"},
			},
		},
		{
			name: "Actions",
			binds: []struct{ key, desc string }{
				{"Space", "Mark/unmark item"},
				{"d", "Delete marked/current"},
				{"E", "Export to JSON"},
				{"r", "Rescan directory"},
			},
		},
		{
			name: "Toggles & General",
			binds: []struct{ key, desc string }{
				{"a", "Apparent / disk size"},
				{".", "Show/hide hidden files"},
				{"?", "Toggle help"},
				{"q", "Quit"},
			},
		},
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "")

	for _, sec := range sections {
		secTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Accent).
			Render("  " + sec.name)
		lines = append(lines, secTitle)

		for _, b := range sec.binds {
			key := lipgloss.NewStyle().
				Foreground(theme.Primary).
				Bold(true).
				Width(14).
				Render("    " + b.key)
			desc := lipgloss.NewStyle().
				Foreground(theme.TextSecondary).
				Render(b.desc)
			lines = append(lines, fmt.Sprintf("%s %s", key, desc))
		}
		lines = append(lines, "")
	}

	close := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Render("  Press ? or Esc to close")
	lines = append(lines, close)

	content := strings.Join(lines, "\n")

	box := theme.ModalStyle.
		Width(boxWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
