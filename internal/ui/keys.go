package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap holds all key bindings for the application.
type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Enter     key.Binding
	Back      key.Binding
	Mark      key.Binding
	Delete    key.Binding
	Export    key.Binding
	Rescan   key.Binding
	Quit      key.Binding
	ForceQuit key.Binding
	Help      key.Binding

	// View switching
	ViewTree     key.Binding
	ViewTreemap  key.Binding
	ViewFileType key.Binding

	// Sort
	SortSize  key.Binding
	SortName  key.Binding
	SortCount key.Binding
	SortMtime key.Binding

	// Toggles
	ToggleApparent key.Binding
	ToggleHidden   key.Binding

	// Confirm dialog
	ConfirmYes key.Binding
	ConfirmNo  key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "parent"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "enter"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "enter dir"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "go back"),
		),
		Mark: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "mark"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Export: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "export"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		ViewTree: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "tree view"),
		),
		ViewTreemap: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "treemap"),
		),
		ViewFileType: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "file types"),
		),
		SortSize: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort: size"),
		),
		SortName: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "sort: name"),
		),
		SortCount: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "sort: count"),
		),
		SortMtime: key.NewBinding(
			key.WithKeys("M"),
			key.WithHelp("M", "sort: mtime"),
		),
		ToggleApparent: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "apparent/disk"),
		),
		ToggleHidden: key.NewBinding(
			key.WithKeys("."),
			key.WithHelp(".", "hidden files"),
		),
		ConfirmYes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "yes"),
		),
		ConfirmNo: key.NewBinding(
			key.WithKeys("n", "N", "esc"),
			key.WithHelp("n/esc", "no"),
		),
	}
}
