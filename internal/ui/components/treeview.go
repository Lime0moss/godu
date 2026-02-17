package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/serdar/godu/internal/model"
	"github.com/serdar/godu/internal/ui/style"
	"github.com/serdar/godu/internal/util"
)

// TreeView renders the main tree list view.
type TreeView struct {
	Theme       style.Theme
	Layout      style.Layout
	Items       []model.TreeNode
	Cursor      int
	Offset      int
	Marked      map[string]bool
	UseApparent bool
	ParentSize  int64
}

// Render renders the tree view.
func (tv *TreeView) Render() string {
	width := tv.Layout.ContentWidth()

	if len(tv.Items) == 0 {
		empty := lipgloss.NewStyle().Foreground(tv.Theme.TextMuted).Render("  (empty directory)")
		return style.FullWidth(empty, width)
	}

	contentHeight := tv.Layout.ContentHeight()
	barWidth := tv.Layout.BarWidth()
	nameWidth := tv.Layout.NameWidth()

	start := tv.Offset
	end := start + contentHeight
	if end > len(tv.Items) {
		end = len(tv.Items)
	}

	var lines []string
	for i := start; i < end; i++ {
		item := tv.Items[i]
		selected := i == tv.Cursor
		marked := tv.Marked[item.Path()]
		line := tv.renderRow(item, selected, marked, barWidth, nameWidth, width)
		lines = append(lines, line)
	}

	// Pad remaining height
	for len(lines) < contentHeight {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (tv *TreeView) renderRow(item model.TreeNode, selected, marked bool, barWidth, nameWidth, totalWidth int) string {
	var size int64
	if tv.UseApparent {
		size = item.GetSize()
	} else {
		size = item.GetUsage()
	}

	// Percentage
	pct := util.Percent(size, tv.ParentSize)
	pctStr := fmt.Sprintf("%5.1f%%", pct)

	// Gradient bar
	ratio := pct / 100.0
	bar := tv.Theme.BarGradient(barWidth, ratio)

	// Name (truncated to fit)
	name := item.GetName()
	if item.IsDir() {
		name += "/"
	}
	name = util.TruncateString(name, nameWidth)

	// Cursor / mark indicator (2 chars)
	indicator := "  "
	if selected && marked {
		indicator = tv.Theme.MarkedIndicator.Render("*") + tv.Theme.CursorIndicator.Render(">")
	} else if selected {
		indicator = tv.Theme.CursorIndicator.Render(" >")
	} else if marked {
		indicator = tv.Theme.MarkedIndicator.Render("* ")
	}

	// Size string
	sizeStr := util.FormatSize(size)

	// Style the name
	var nameStyled string
	if item.IsDir() {
		nameStyled = tv.Theme.DirName.Render(name)
	} else {
		nameStyled = tv.Theme.FileName.Render(name)
	}

	// Flag indicators (appended to name but counted in nameWidth)
	flag := item.GetFlag()
	if flag&model.FlagError != 0 {
		nameStyled += tv.Theme.ErrorText.Render(" !")
	}
	if flag&model.FlagSymlink != 0 {
		nameStyled += lipgloss.NewStyle().Foreground(tv.Theme.TextMuted).Render(" ->")
	}

	// Styled components
	pctStyled := tv.Theme.PercentText.Render(pctStr)
	sizeStyled := tv.Theme.SizeText.Width(10).Render(sizeStr)

	// Build the row — each segment is a known visual width
	row := fmt.Sprintf("%s%s [%s] %s %s",
		indicator, pctStyled, bar, nameStyled, sizeStyled,
	)

	// Ensure exactly totalWidth visual chars (pad or don't exceed)
	row = style.FullWidth(row, totalWidth)

	if selected {
		return tv.Theme.SelectedRow.Width(totalWidth).Render(row)
	}
	return row
}

// EnsureVisible adjusts offset to keep cursor visible.
func (tv *TreeView) EnsureVisible() {
	contentHeight := tv.Layout.ContentHeight()
	if tv.Cursor < tv.Offset {
		tv.Offset = tv.Cursor
	}
	if tv.Cursor >= tv.Offset+contentHeight {
		tv.Offset = tv.Cursor - contentHeight + 1
	}
	if tv.Offset < 0 {
		tv.Offset = 0
	}
}
