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

type rect struct {
	x, y, w, h int
}

type treemapItem struct {
	node model.TreeNode
	size int64
}

// RenderTreemap renders a squarified treemap visualization.
func RenderTreemap(theme style.Theme, dir *model.DirNode, useApparent bool, showHidden bool, width, height int) string {
	if dir == nil || height <= 0 || width <= 0 {
		return ""
	}

	children := dir.GetChildren()
	if !showHidden {
		var filtered []model.TreeNode
		for _, c := range children {
			if len(c.GetName()) > 0 && c.GetName()[0] != '.' {
				filtered = append(filtered, c)
			}
		}
		children = filtered
	}
	if len(children) == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Render("  (empty directory)")
	}

	var items []treemapItem
	var totalSize int64
	for _, c := range children {
		var sz int64
		if useApparent {
			sz = c.GetSize()
		} else {
			sz = c.GetUsage()
		}
		if sz > 0 {
			items = append(items, treemapItem{node: c, size: sz})
			totalSize += sz
		}
	}

	if len(items) == 0 {
		return lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Render("  (no items with size)")
	}

	// Sort descending
	sort.Slice(items, func(i, j int) bool { return items[i].size > items[j].size })

	maxItems := (width * height) / 8
	if maxItems < 5 {
		maxItems = 5
	}
	if len(items) > maxItems {
		var otherSize int64
		for i := maxItems - 1; i < len(items); i++ {
			otherSize += items[i].size
		}
		items = items[:maxItems-1]
		items = append(items, treemapItem{node: nil, size: otherSize})
	}

	// Create grid
	grid := make([][]rune, height)
	colorGrid := make([][]lipgloss.Color, height)
	for y := 0; y < height; y++ {
		grid[y] = make([]rune, width)
		colorGrid[y] = make([]lipgloss.Color, width)
		for x := 0; x < width; x++ {
			grid[y][x] = ' '
			colorGrid[y][x] = theme.BgDark
		}
	}

	rects := squarify(items, totalSize, rect{0, 0, width, height})

	for i, r := range rects {
		if r.w <= 0 || r.h <= 0 {
			continue
		}

		var color lipgloss.Color
		if i < len(items) && items[i].node != nil {
			cat := model.ClassifyFile(items[i].node.GetName())
			if items[i].node.IsDir() {
				color = theme.Accent
			} else {
				color = lipgloss.Color(model.CategoryColor(cat))
			}
		} else {
			color = theme.Muted
		}

		fillRect(grid, colorGrid, r, color)
		drawBorder(grid, r)

		if i < len(items) {
			var label string
			if items[i].node != nil {
				name := items[i].node.GetName()
				if items[i].node.IsDir() {
					name += "/"
				}
				sz := util.FormatSize(items[i].size)
				label = fmt.Sprintf("%s %s", name, sz)
			} else {
				label = fmt.Sprintf("other (%s)", util.FormatSize(items[i].size))
			}
			placeLabel(grid, r, label)
		}
	}

	var lines []string
	for y := 0; y < height; y++ {
		var line strings.Builder
		for x := 0; x < width; x++ {
			ch := grid[y][x]
			color := colorGrid[y][x]
			if ch == ' ' || ch == '\x00' {
				s := lipgloss.NewStyle().Background(color)
				line.WriteString(s.Render(" "))
			} else {
				s := lipgloss.NewStyle().Foreground(theme.TextPrimary)
				line.WriteString(s.Render(string(ch)))
			}
		}
		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n")
}

func squarify(items []treemapItem, totalSize int64, bounds rect) []rect {
	result := make([]rect, len(items))
	if len(items) == 0 || bounds.w <= 0 || bounds.h <= 0 {
		return result
	}
	layoutRow(items, result, 0, len(items), totalSize, bounds)
	return result
}

func layoutRow(items []treemapItem, result []rect, start, end int, totalSize int64, bounds rect) {
	if start >= end || bounds.w <= 0 || bounds.h <= 0 || totalSize <= 0 {
		return
	}

	if end-start == 1 {
		result[start] = bounds
		return
	}

	horizontal := bounds.w >= bounds.h

	runningSize := int64(0)
	bestSplit := start + 1
	bestRatio := float64(1e18)

	for i := start; i < end-1; i++ {
		runningSize += items[i].size
		fraction := float64(runningSize) / float64(totalSize)

		var dim1, dim2 float64
		if horizontal {
			dim1 = fraction * float64(bounds.w)
			dim2 = float64(bounds.h)
		} else {
			dim1 = float64(bounds.w)
			dim2 = fraction * float64(bounds.h)
		}

		aspect := dim1 / dim2
		if dim2 > dim1 {
			aspect = dim2 / dim1
		}

		if aspect < bestRatio {
			bestRatio = aspect
			bestSplit = i + 1
		}
	}

	var leftSize int64
	for i := start; i < bestSplit; i++ {
		leftSize += items[i].size
	}
	fraction := float64(leftSize) / float64(totalSize)

	var leftBounds, rightBounds rect
	if horizontal {
		splitX := int(fraction * float64(bounds.w))
		if splitX < 1 {
			splitX = 1
		}
		if splitX >= bounds.w {
			splitX = bounds.w - 1
		}
		leftBounds = rect{bounds.x, bounds.y, splitX, bounds.h}
		rightBounds = rect{bounds.x + splitX, bounds.y, bounds.w - splitX, bounds.h}
	} else {
		splitY := int(fraction * float64(bounds.h))
		if splitY < 1 {
			splitY = 1
		}
		if splitY >= bounds.h {
			splitY = bounds.h - 1
		}
		leftBounds = rect{bounds.x, bounds.y, bounds.w, splitY}
		rightBounds = rect{bounds.x, bounds.y + splitY, bounds.w, bounds.h - splitY}
	}

	rightSize := totalSize - leftSize
	layoutRow(items, result, start, bestSplit, leftSize, leftBounds)
	layoutRow(items, result, bestSplit, end, rightSize, rightBounds)
}

func fillRect(grid [][]rune, colorGrid [][]lipgloss.Color, r rect, color lipgloss.Color) {
	for y := r.y; y < r.y+r.h && y < len(grid); y++ {
		for x := r.x; x < r.x+r.w && x < len(grid[y]); x++ {
			grid[y][x] = ' '
			colorGrid[y][x] = color
		}
	}
}

func drawBorder(grid [][]rune, r rect) {
	if r.w < 2 || r.h < 2 {
		return
	}
	h := len(grid)
	w := len(grid[0])

	for x := r.x; x < r.x+r.w && x < w; x++ {
		if r.y < h {
			if x == r.x {
				grid[r.y][x] = '┌'
			} else if x == r.x+r.w-1 {
				grid[r.y][x] = '┐'
			} else {
				grid[r.y][x] = '─'
			}
		}
		by := r.y + r.h - 1
		if by < h {
			if x == r.x {
				grid[by][x] = '└'
			} else if x == r.x+r.w-1 {
				grid[by][x] = '┘'
			} else {
				grid[by][x] = '─'
			}
		}
	}

	for y := r.y + 1; y < r.y+r.h-1 && y < h; y++ {
		if r.x < w {
			grid[y][r.x] = '│'
		}
		rx := r.x + r.w - 1
		if rx < w {
			grid[y][rx] = '│'
		}
	}
}

func placeLabel(grid [][]rune, r rect, label string) {
	innerW := r.w - 2
	innerH := r.h - 2
	if innerW <= 0 || innerH <= 0 {
		return
	}

	runes := []rune(label)
	if len(runes) > innerW {
		if innerW > 3 {
			runes = append(runes[:innerW-3], '.', '.', '.')
		} else {
			runes = runes[:innerW]
		}
	}

	y := r.y + 1
	x := r.x + 1
	if y < len(grid) {
		for i, ch := range runes {
			if x+i < len(grid[y]) {
				grid[y][x+i] = ch
			}
		}
	}
}
