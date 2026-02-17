package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sadopc/godu/internal/ops"
	"github.com/sadopc/godu/internal/scanner"
	"github.com/sadopc/godu/internal/ui"
)

var (
	version = "dev"
)

func main() {
	// Flags
	exportPath := flag.String("export", "", "Export scan results to JSON file (headless mode)")
	importPath := flag.String("import", "", "Import and view scan results from JSON file")
	showHidden := flag.Bool("hidden", true, "Show hidden files")
	noHidden := flag.Bool("no-hidden", false, "Hide hidden files")
	showVersion := flag.Bool("version", false, "Show version")
	disableGC := flag.Bool("no-gc", false, "Disable GC during scan (faster but uses more memory)")
	exclude := flag.String("exclude", "", "Comma-separated list of directory names to exclude")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "godu - Interactive disk usage analyzer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: godu [options] [path]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  godu .                    Scan current directory\n")
		fmt.Fprintf(os.Stderr, "  godu /home                Scan /home\n")
		fmt.Fprintf(os.Stderr, "  godu --export scan.json . Export scan to JSON\n")
		fmt.Fprintf(os.Stderr, "  godu --import scan.json   View exported scan\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("godu %s\n", version)
		os.Exit(0)
	}

	// Import mode
	if *importPath != "" {
		if *exportPath != "" {
			// Re-export an imported scan
			root, err := ops.ImportJSON(*importPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error importing: %v\n", err)
				os.Exit(1)
			}
			if err := ops.ExportJSON(root, *exportPath, version); err != nil {
				fmt.Fprintf(os.Stderr, "Error exporting: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Exported to %s\n", *exportPath)
			os.Exit(0)
		}

		app := ui.NewAppFromImport(*importPath)
		app.Version = version
		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Determine scan path
	scanPath := "."
	if flag.NArg() > 0 {
		scanPath = flag.Arg(0)
	}

	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Verify path exists
	info, err := os.Stat(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", absPath)
		os.Exit(1)
	}

	// Build scan options
	opts := scanner.DefaultOptions()
	opts.ShowHidden = *showHidden
	if *noHidden {
		opts.ShowHidden = false
	}
	opts.DisableGC = *disableGC

	if *exclude != "" {
		for _, e := range splitComma(*exclude) {
			if e != "" {
				opts.ExcludePatterns = append(opts.ExcludePatterns, e)
			}
		}
	}

	// Headless export mode
	if *exportPath != "" {
		fmt.Printf("Scanning %s...\n", absPath)
		s := scanner.NewParallelScanner()
		root, err := s.Scan(context.Background(), absPath, opts, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
			os.Exit(1)
		}
		if err := ops.ExportJSON(root, *exportPath, version); err != nil {
			fmt.Fprintf(os.Stderr, "Export error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Exported to %s\n", *exportPath)
		return
	}

	// Interactive TUI mode
	app := ui.NewApp(absPath, opts)
	app.ExportPath = "godu-export.json"
	app.Version = version

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func splitComma(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
