package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/serdar/godu/internal/model"
	"github.com/serdar/godu/internal/ops"
	"github.com/serdar/godu/internal/scanner"
	"github.com/serdar/godu/internal/ui/components"
	"github.com/serdar/godu/internal/ui/style"
)

// ViewMode represents the current view.
type ViewMode int

const (
	ViewTree ViewMode = iota
	ViewTreemap
	ViewFileType
)

// AppState represents the application state.
type AppState int

const (
	StateScanning AppState = iota
	StateBrowsing
	StateConfirmDelete
	StateHelp
	StateExporting
)

// ScanDoneMsg is sent when scanning completes.
type ScanDoneMsg struct {
	Root *model.DirNode
	Err  error
}

// ProgressMsg carries scanner progress updates.
type ProgressMsg scanner.Progress

// DeleteDoneMsg is sent when deletion completes.
type DeleteDoneMsg struct {
	Deleted []string
	Errors  []error
}

// ExportDoneMsg is sent when export completes.
type ExportDoneMsg struct {
	Path string
	Err  error
}

type tickMsg time.Time

// App is the root Bubble Tea model.
type App struct {
	ScanPath    string
	ScanOptions scanner.ScanOptions
	ImportPath  string
	ExportPath  string
	Version     string

	state    AppState
	viewMode ViewMode
	width    int
	height   int

	root        *model.DirNode
	currentDir  *model.DirNode
	navStack    []*model.DirNode
	sortConfig  model.SortConfig
	sortedItems []model.TreeNode

	cursor int
	offset int

	marked      map[string]bool
	markedItems []components.ConfirmItem

	useApparent bool
	showHidden  bool
	imported    bool

	scanProgress   scanner.Progress
	progressMu     sync.Mutex
	latestProgress scanner.Progress
	scanCancel     context.CancelFunc
	scanCancelMu   sync.Mutex

	theme  style.Theme
	keys   KeyMap
	layout style.Layout

	errorMsg string
}

func (a *App) setScanCancel(cancel context.CancelFunc) {
	a.scanCancelMu.Lock()
	a.scanCancel = cancel
	a.scanCancelMu.Unlock()
}

func (a *App) callScanCancel() {
	a.scanCancelMu.Lock()
	if a.scanCancel != nil {
		a.scanCancel()
	}
	a.scanCancelMu.Unlock()
}

// NewApp creates a new App model.
func NewApp(scanPath string, opts scanner.ScanOptions) *App {
	return &App{
		ScanPath:    scanPath,
		ScanOptions: opts,
		state:       StateScanning,
		viewMode:    ViewTree,
		sortConfig:  model.DefaultSort(),
		marked:      make(map[string]bool),
		useApparent: false,
		showHidden:  opts.ShowHidden,
		theme:       style.DefaultTheme(),
		keys:        DefaultKeyMap(),
	}
}

// NewAppFromImport creates an App that loads from a JSON file.
func NewAppFromImport(importPath string) *App {
	return &App{
		ImportPath:  importPath,
		state:       StateScanning,
		viewMode:    ViewTree,
		sortConfig:  model.DefaultSort(),
		marked:      make(map[string]bool),
		useApparent: false,
		showHidden:  true,
		imported:    true,
		theme:       style.DefaultTheme(),
		keys:        DefaultKeyMap(),
	}
}

func (a *App) Init() tea.Cmd {
	if a.ImportPath != "" {
		return a.importCmd()
	}
	// Start both the scan AND the progress ticker simultaneously
	return tea.Batch(a.scanCmd(), a.tickCmd())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layout = style.NewLayout(msg.Width, msg.Height)
		return a, nil

	case ScanDoneMsg:
		if msg.Err != nil {
			a.errorMsg = msg.Err.Error()
			return a, tea.Quit
		}
		a.root = msg.Root
		a.currentDir = msg.Root
		a.state = StateBrowsing
		a.refreshSorted()
		return a, nil

	case tickMsg:
		if a.state == StateScanning {
			// Read latest progress snapshot
			a.progressMu.Lock()
			a.scanProgress = a.latestProgress
			a.progressMu.Unlock()
			// Keep ticking while scanning
			return a, a.tickCmd()
		}
		return a, nil

	case DeleteDoneMsg:
		a.state = StateBrowsing
		a.clearMarks()
		a.refreshSorted()
		if a.cursor >= len(a.sortedItems) {
			a.cursor = len(a.sortedItems) - 1
		}
		if a.cursor < 0 {
			a.cursor = 0
		}
		if len(msg.Errors) > 0 {
			a.errorMsg = fmt.Sprintf("Delete: %d failed (%v)", len(msg.Errors), msg.Errors[0])
		} else if len(msg.Deleted) > 0 {
			a.errorMsg = fmt.Sprintf("Deleted %d item(s)", len(msg.Deleted))
		}
		return a, nil

	case ExportDoneMsg:
		a.state = StateBrowsing
		if msg.Err != nil {
			a.errorMsg = fmt.Sprintf("Export failed: %v", msg.Err)
		} else {
			a.errorMsg = fmt.Sprintf("Exported to %s", msg.Path)
		}
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, a.keys.ForceQuit) {
		a.callScanCancel()
		return a, tea.Quit
	}

	switch a.state {
	case StateScanning:
		if key.Matches(msg, a.keys.Quit) {
			a.callScanCancel()
			return a, tea.Quit
		}
		return a, nil

	case StateHelp:
		if key.Matches(msg, a.keys.Help) || msg.String() == "esc" {
			a.state = StateBrowsing
		}
		return a, nil

	case StateConfirmDelete:
		if key.Matches(msg, a.keys.ConfirmYes) {
			return a, a.executeDelete()
		}
		if key.Matches(msg, a.keys.ConfirmNo) {
			a.state = StateBrowsing
		}
		return a, nil

	case StateBrowsing:
		return a.handleBrowsingKey(msg)
	}

	return a, nil
}

func (a *App) handleBrowsingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a.errorMsg = ""
	switch {
	case key.Matches(msg, a.keys.Quit):
		return a, tea.Quit

	case key.Matches(msg, a.keys.Help):
		a.state = StateHelp
		return a, nil

	case key.Matches(msg, a.keys.Up):
		a.moveCursor(-1)
	case key.Matches(msg, a.keys.Down):
		a.moveCursor(1)
	case key.Matches(msg, a.keys.Enter), key.Matches(msg, a.keys.Right):
		a.enterDir()
	case key.Matches(msg, a.keys.Left), key.Matches(msg, a.keys.Back):
		a.goBack()

	case key.Matches(msg, a.keys.ViewTree):
		a.viewMode = ViewTree
	case key.Matches(msg, a.keys.ViewTreemap):
		a.viewMode = ViewTreemap
	case key.Matches(msg, a.keys.ViewFileType):
		a.viewMode = ViewFileType

	case key.Matches(msg, a.keys.SortSize):
		a.toggleSort(model.SortBySize)
	case key.Matches(msg, a.keys.SortName):
		a.toggleSort(model.SortByName)
	case key.Matches(msg, a.keys.SortCount):
		a.toggleSort(model.SortByCount)
	case key.Matches(msg, a.keys.SortMtime):
		a.toggleSort(model.SortByMtime)

	case key.Matches(msg, a.keys.ToggleApparent):
		a.useApparent = !a.useApparent
		a.refreshSorted()
	case key.Matches(msg, a.keys.ToggleHidden):
		a.showHidden = !a.showHidden
		a.ScanOptions.ShowHidden = a.showHidden
		a.clearMarks()
		a.refreshSorted()

	case key.Matches(msg, a.keys.Mark):
		a.toggleMark()

	case key.Matches(msg, a.keys.Delete):
		return a, a.prepareDelete()

	case key.Matches(msg, a.keys.Export):
		return a, a.exportCmd()

	case key.Matches(msg, a.keys.Rescan):
		a.clearMarks()
		a.state = StateScanning
		return a, tea.Batch(a.scanCmd(), a.tickCmd())
	}

	return a, nil
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	switch a.state {
	case StateScanning:
		return components.RenderScanProgress(a.theme, a.scanProgress, a.width, a.height)

	case StateHelp:
		return components.RenderHelp(a.theme, a.width, a.height)

	case StateConfirmDelete:
		return components.RenderConfirmDialog(a.theme, a.markedItems, a.width, a.height)

	case StateBrowsing, StateExporting:
		return a.renderBrowsing()
	}

	return ""
}

func (a *App) renderBrowsing() string {
	header := components.RenderHeader(a.theme, a.root, a.width)
	breadcrumb := components.RenderBreadcrumb(a.theme, a.currentDir, a.width)
	tabBar := components.RenderTabBar(a.theme, int(a.viewMode), a.sortConfig.Field, a.width)

	var content string
	switch a.viewMode {
	case ViewTree:
		tv := &components.TreeView{
			Theme:       a.theme,
			Layout:      a.layout,
			Items:       a.sortedItems,
			Cursor:      a.cursor,
			Offset:      a.offset,
			Marked:      a.marked,
			UseApparent: a.useApparent,
			ParentSize:  a.getParentSize(),
		}
		tv.EnsureVisible()
		a.offset = tv.Offset
		content = tv.Render()

	case ViewTreemap:
		content = components.RenderTreemap(a.theme, a.currentDir, a.useApparent, a.showHidden, a.layout.ContentWidth(), a.layout.ContentHeight())

	case ViewFileType:
		content = components.RenderFileTypes(a.theme, a.currentDir, a.useApparent, a.showHidden, a.layout.ContentWidth(), a.layout.ContentHeight())
	}

	statusInfo := components.StatusInfo{
		CurrentDir:  a.currentDir,
		MarkedCount: len(a.marked),
		UseApparent: a.useApparent,
		ShowHidden:  a.showHidden,
		SortField:   a.sortConfig.Field,
		ViewMode:    int(a.viewMode),
		ErrorMsg:    a.errorMsg,
	}
	for markedPath := range a.marked {
		for _, item := range a.sortedItems {
			if item.Path() == markedPath {
				statusInfo.MarkedSize += item.GetSize()
			}
		}
	}
	statusBar := components.RenderStatusBar(a.theme, statusInfo, a.width)

	return header + "\n" + breadcrumb + "\n" + tabBar + "\n" + content + "\n" + statusBar
}

func (a *App) moveCursor(delta int) {
	a.cursor += delta
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor >= len(a.sortedItems) {
		a.cursor = len(a.sortedItems) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

func (a *App) enterDir() {
	if a.cursor >= len(a.sortedItems) {
		return
	}
	item := a.sortedItems[a.cursor]
	if dir, ok := item.(*model.DirNode); ok {
		a.navStack = append(a.navStack, a.currentDir)
		a.currentDir = dir
		a.cursor = 0
		a.offset = 0
		a.clearMarks()
		a.refreshSorted()
	}
}

func (a *App) goBack() {
	if len(a.navStack) == 0 {
		return
	}
	prev := a.navStack[len(a.navStack)-1]
	a.navStack = a.navStack[:len(a.navStack)-1]

	leavingName := a.currentDir.Name
	a.currentDir = prev
	a.clearMarks()
	a.refreshSorted()

	for i, item := range a.sortedItems {
		if item.GetName() == leavingName {
			a.cursor = i
			break
		}
	}
	a.offset = 0
}

func (a *App) toggleSort(field model.SortField) {
	if a.sortConfig.Field == field {
		if a.sortConfig.Order == model.SortDesc {
			a.sortConfig.Order = model.SortAsc
		} else {
			a.sortConfig.Order = model.SortDesc
		}
	} else {
		a.sortConfig.Field = field
		a.sortConfig.Order = model.SortDesc
	}
	a.refreshSorted()
}

func (a *App) toggleMark() {
	if a.cursor >= len(a.sortedItems) {
		return
	}
	p := a.sortedItems[a.cursor].Path()
	if a.marked[p] {
		delete(a.marked, p)
	} else {
		a.marked[p] = true
	}
	a.moveCursor(1)
}

func (a *App) clearMarks() {
	a.marked = make(map[string]bool)
}

func (a *App) refreshSorted() {
	if a.currentDir == nil {
		a.sortedItems = nil
		return
	}
	children := a.currentDir.GetChildren()

	if !a.showHidden {
		var filtered []model.TreeNode
		for _, c := range children {
			if len(c.GetName()) > 0 && c.GetName()[0] != '.' {
				filtered = append(filtered, c)
			}
		}
		children = filtered
	}

	model.SortChildren(children, a.sortConfig, a.useApparent)
	a.sortedItems = children
}

func (a *App) getParentSize() int64 {
	if a.currentDir == nil {
		return 0
	}
	if a.useApparent {
		return a.currentDir.GetSize()
	}
	return a.currentDir.GetUsage()
}

// scanCmd runs the directory scan in a background goroutine.
// Progress is communicated via a.latestProgress (mutex-protected).
func (a *App) scanCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		a.setScanCancel(cancel)

		progressCh := make(chan scanner.Progress, 10)

		// Relay progress updates to shared state (read by tickMsg handler)
		go func() {
			for p := range progressCh {
				a.progressMu.Lock()
				a.latestProgress = p
				a.progressMu.Unlock()
			}
		}()

		s := scanner.NewParallelScanner()
		root, err := s.Scan(ctx, a.ScanPath, a.ScanOptions, progressCh)
		close(progressCh)

		return ScanDoneMsg{Root: root, Err: err}
	}
}

func (a *App) importCmd() tea.Cmd {
	return func() tea.Msg {
		root, err := ops.ImportJSON(a.ImportPath)
		return ScanDoneMsg{Root: root, Err: err}
	}
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(60*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) prepareDelete() tea.Cmd {
	if a.imported {
		a.errorMsg = "Delete is disabled in import mode"
		return nil
	}
	if a.currentDir == nil {
		return nil
	}

	var items []components.ConfirmItem

	if len(a.marked) > 0 {
		for markedPath := range a.marked {
			for _, item := range a.sortedItems {
				if item.Path() == markedPath {
					items = append(items, components.ConfirmItem{
						Name:  item.GetName(),
						Path:  item.Path(),
						Size:  item.GetSize(),
						IsDir: item.IsDir(),
					})
				}
			}
		}
	} else if a.cursor < len(a.sortedItems) {
		item := a.sortedItems[a.cursor]
		items = append(items, components.ConfirmItem{
			Name:  item.GetName(),
			Path:  item.Path(),
			Size:  item.GetSize(),
			IsDir: item.IsDir(),
		})
	}

	if len(items) == 0 {
		return nil
	}

	a.markedItems = items
	a.state = StateConfirmDelete
	return nil
}

func (a *App) executeDelete() tea.Cmd {
	items := a.markedItems
	currentDir := a.currentDir
	rootPath := a.root.Path()

	return func() tea.Msg {
		var deleted []string
		var errors []error

		for _, item := range items {
			err := ops.Delete(item.Path, rootPath)
			if err != nil {
				errors = append(errors, err)
			} else {
				deleted = append(deleted, item.Name)
				currentDir.RemoveChild(item.Name)
			}
		}

		return DeleteDoneMsg{Deleted: deleted, Errors: errors}
	}
}

func (a *App) exportCmd() tea.Cmd {
	if a.root == nil {
		return nil
	}

	exportPath := a.ExportPath
	if exportPath == "" {
		exportPath = "godu-export.json"
	}

	a.state = StateExporting
	root := a.root

	version := a.Version
	return func() tea.Msg {
		err := ops.ExportJSON(root, exportPath, version)
		return ExportDoneMsg{Path: exportPath, Err: err}
	}
}
