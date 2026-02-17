package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/ui/style"
	"github.com/sadopc/godu/internal/util"
)

// CategoryStats holds aggregated stats for a file category.
type CategoryStats struct {
	Category  model.FileCategory
	FileCount int64
	TotalSize int64
	TopExts   map[string]int64
}

// ftCache caches the result of aggregateFileTypes to avoid recomputing on every render.
type ftCache struct {
	dir         *model.DirNode
	useApparent bool
	showHidden  bool
	stats       []CategoryStats
}

var lastFTCache ftCache

// InvalidateFileTypeCache clears the cached file type aggregation,
// forcing a recompute on the next render.
func InvalidateFileTypeCache() {
	lastFTCache = ftCache{}
}

// RenderFileTypes renders the file type breakdown view.
func RenderFileTypes(theme style.Theme, dir *model.DirNode, useApparent bool, showHidden bool, width, height int) string {
	if dir == nil {
		return ""
	}

	var stats []CategoryStats
	if lastFTCache.dir == dir && lastFTCache.useApparent == useApparent && lastFTCache.showHidden == showHidden {
		stats = lastFTCache.stats
	} else {
		stats = aggregateFileTypes(dir, useApparent, showHidden)
		lastFTCache = ftCache{dir: dir, useApparent: useApparent, showHidden: showHidden, stats: stats}
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TotalSize > stats[j].TotalSize
	})

	var totalSize int64
	for _, s := range stats {
		totalSize += s.TotalSize
	}

	if totalSize == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Render("  (no files found)")
	}

	catW := 14
	countW := 10
	sizeW := 12
	barW := width - catW - countW - sizeW - 10
	if barW < 10 {
		barW = 10
	}
	if barW > 30 {
		barW = 30
	}

	var lines []string

	hdrStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.TextPrimary)
	header := fmt.Sprintf("  %-*s %*s %*s  %s",
		catW, "Category",
		countW, "Files",
		sizeW, "Size",
		"Distribution",
	)
	lines = append(lines, hdrStyle.Render(header))

	sep := lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  " + strings.Repeat("-", max(width-4, 0)))
	lines = append(lines, sep)

	for _, s := range stats {
		pct := util.Percent(s.TotalSize, totalSize)
		ratio := pct / 100.0

		catColor := lipgloss.Color(model.CategoryColor(s.Category))
		catName := lipgloss.NewStyle().Foreground(catColor).Bold(true).Width(catW).Render(model.CategoryName(s.Category))
		count := lipgloss.NewStyle().Foreground(theme.TextSecondary).Width(countW).Align(lipgloss.Right).Render(util.FormatCount(s.FileCount))
		size := lipgloss.NewStyle().Foreground(theme.TextSecondary).Width(sizeW).Align(lipgloss.Right).Render(util.FormatSize(s.TotalSize))

		bar := renderCategoryBar(barW, ratio, catColor, theme.TextMuted)
		pctStr := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(fmt.Sprintf(" %5.1f%%", pct))

		row := fmt.Sprintf("  %s %s %s  %s%s", catName, count, size, bar, pctStr)
		lines = append(lines, row)

		topExts := getTopExtensions(s.TopExts, 3)
		if len(topExts) > 0 {
			extStr := lipgloss.NewStyle().Foreground(theme.TextMuted).
				Render("    " + strings.Join(topExts, ", "))
			lines = append(lines, extStr)
		}
	}

	lines = append(lines, sep)

	totalLine := fmt.Sprintf("  %-*s %*s %*s",
		catW, "Total",
		countW, "",
		sizeW, util.FormatSize(totalSize),
	)
	lines = append(lines, hdrStyle.Render(totalLine))

	for len(lines) < height {
		lines = append(lines, "")
	}

	// Apply explicit background to every line so treemap colors don't bleed through.
	bgStyle := lipgloss.NewStyle().
		Background(theme.BgDark).
		Width(width)
	for i := range lines[:height] {
		lines[i] = bgStyle.Render(lines[i])
	}

	return strings.Join(lines[:height], "\n")
}

func aggregateFileTypes(dir *model.DirNode, useApparent bool, showHidden bool) []CategoryStats {
	catMap := make(map[model.FileCategory]*CategoryStats)

	var walk func(d *model.DirNode)
	walk = func(d *model.DirNode) {
		for _, child := range d.GetChildren() {
			name := child.GetName()
			if !showHidden && len(name) > 0 && name[0] == '.' {
				continue
			}
			if cd, ok := child.(*model.DirNode); ok {
				walk(cd)
			} else {
				cat := model.ClassifyFile(child.GetName())
				ext := model.GetExtension(child.GetName())

				var sz int64
				if useApparent {
					sz = child.GetSize()
				} else {
					sz = child.GetUsage()
				}

				st, ok := catMap[cat]
				if !ok {
					st = &CategoryStats{
						Category: cat,
						TopExts:  make(map[string]int64),
					}
					catMap[cat] = st
				}
				st.FileCount++
				st.TotalSize += sz
				if ext != "" {
					st.TopExts[ext] += sz
				}
			}
		}
	}

	walk(dir)

	result := make([]CategoryStats, 0, len(catMap))
	for _, s := range catMap {
		result = append(result, *s)
	}
	return result
}

func getTopExtensions(exts map[string]int64, n int) []string {
	type extEntry struct {
		ext  string
		size int64
	}
	var entries []extEntry
	for ext, size := range exts {
		entries = append(entries, extEntry{ext, size})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].size > entries[j].size
	})

	var result []string
	for i := 0; i < n && i < len(entries); i++ {
		result = append(result, fmt.Sprintf("%s (%s)", entries[i].ext, util.FormatSize(entries[i].size)))
	}
	return result
}

func renderCategoryBar(width int, ratio float64, color, dimColor lipgloss.Color) string {
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}

	var buf strings.Builder
	filledStyle := lipgloss.NewStyle().Foreground(color)
	dimStyle := lipgloss.NewStyle().Foreground(dimColor)

	for i := 0; i < filled; i++ {
		buf.WriteString(filledStyle.Render("="))
	}
	for i := filled; i < width; i++ {
		buf.WriteString(dimStyle.Render("-"))
	}
	return buf.String()
}
