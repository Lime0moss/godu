package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/serdar/godu/internal/scanner"
	"github.com/serdar/godu/internal/ui/style"
	"github.com/serdar/godu/internal/util"
)

// RenderScanProgress renders the scanning progress overlay.
func RenderScanProgress(theme style.Theme, progress scanner.Progress, width, height int) string {
	boxWidth := 50
	if boxWidth > width-4 {
		boxWidth = width - 4
	}

	var lines []string

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Render("  Scanning...")

	lines = append(lines, title)
	lines = append(lines, "")

	filesLine := fmt.Sprintf("  Files:  %s", util.FormatCount(progress.FilesScanned))
	dirsLine := fmt.Sprintf("  Dirs:   %s", util.FormatCount(progress.DirsScanned))
	sizeLine := fmt.Sprintf("  Size:   %s", util.FormatSize(progress.BytesFound))
	speedLine := fmt.Sprintf("  Speed:  %s items/s", util.FormatCount(int64(progress.ItemsPerSecond())))

	statStyle := lipgloss.NewStyle().Foreground(theme.TextSecondary)
	lines = append(lines, statStyle.Render(filesLine))
	lines = append(lines, statStyle.Render(dirsLine))
	lines = append(lines, statStyle.Render(sizeLine))
	lines = append(lines, statStyle.Render(speedLine))

	if progress.Errors > 0 {
		errLine := fmt.Sprintf("  Errors: %d", progress.Errors)
		lines = append(lines, theme.ErrorText.Render(errLine))
	}

	lines = append(lines, "")

	elapsed := fmt.Sprintf("  Elapsed: %.1fs", progress.Duration.Seconds())
	lines = append(lines, lipgloss.NewStyle().Foreground(theme.TextMuted).Render(elapsed))

	content := strings.Join(lines, "\n")

	box := theme.ModalStyle.
		Width(boxWidth).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
