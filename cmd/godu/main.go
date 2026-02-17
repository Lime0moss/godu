package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sadopc/godu/internal/ops"
	"github.com/sadopc/godu/internal/remote"
	"github.com/sadopc/godu/internal/scanner"
	"github.com/sadopc/godu/internal/ui"
)

var (
	version = "dev"
)

const defaultSSHPort = 22

type scanTarget struct {
	Remote         bool
	LocalPath      string
	SSHDestination string
	RemotePath     string
}

func main() {
	// Flags
	exportPath := flag.String("export", "", "Export scan results to JSON file (headless mode, use '-' for stdout)")
	importPath := flag.String("import", "", "Import and view scan results from JSON file")
	showHidden := flag.Bool("hidden", true, "Show hidden files")
	noHidden := flag.Bool("no-hidden", false, "Hide hidden files")
	showVersion := flag.Bool("version", false, "Show version")
	disableGC := flag.Bool("no-gc", false, "Disable GC during scan (faster but uses more memory)")
	exclude := flag.String("exclude", "", "Comma-separated list of directory names to exclude")
	followSymlinks := flag.Bool("follow-symlinks", false, "Follow symbolic links during scan")
	concurrency := flag.Int("j", 0, "Max concurrent directory scans (0 = auto: 3x CPU cores)")
	sshPort := flag.Int("ssh-port", defaultSSHPort, "SSH port for remote scans")
	sshBatch := flag.Bool("ssh-batch", false, "Disable SSH password prompts (key/agent auth only)")
	sshTimeout := flag.Int("ssh-timeout", 15, "SSH connection timeout in seconds (default 15)")
	sshScanTimeout := flag.Int("ssh-scan-timeout", 0, "SSH scan timeout in seconds (0 = no limit)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "godu - Interactive disk usage analyzer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: godu [options] [path|user@host [remote-path]]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  godu .                          Scan current directory\n")
		fmt.Fprintf(os.Stderr, "  godu /home                      Scan /home\n")
		fmt.Fprintf(os.Stderr, "  godu --export scan.json .       Export scan to JSON\n")
		fmt.Fprintf(os.Stderr, "  godu --import scan.json         View exported scan\n")
		fmt.Fprintf(os.Stderr, "  godu user@192.168.1.10          Scan remote home directory over SSH\n")
		fmt.Fprintf(os.Stderr, "  godu --ssh-port 2222 user@host /var/log\n")
		fmt.Fprintf(os.Stderr, "  godu --ssh-batch user@host      Key-based/agent auth only (no password prompt)\n")
		fmt.Fprintf(os.Stderr, "  godu --follow-symlinks .        Follow symlinks during scan\n")
		fmt.Fprintf(os.Stderr, "  godu -j 8 /home                 Scan with 8 concurrent workers\n")
	}

	flag.Parse()

	// Detect conflicting --hidden / --no-hidden flags
	hiddenSet, noHiddenSet := false, false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "hidden" {
			hiddenSet = true
		}
		if f.Name == "no-hidden" {
			noHiddenSet = true
		}
	})
	if hiddenSet && noHiddenSet {
		fmt.Fprintf(os.Stderr, "Error: --hidden and --no-hidden cannot be used together\n")
		os.Exit(1)
	}

	if *showVersion {
		fmt.Printf("godu %s\n", version)
		os.Exit(0)
	}

	if *sshPort < 1 || *sshPort > 65535 {
		fmt.Fprintf(os.Stderr, "Error: ssh-port must be between 1 and 65535\n")
		os.Exit(1)
	}

	// Import mode
	if *importPath != "" {
		if flag.NArg() > 0 {
			fmt.Fprintf(os.Stderr, "Error: --import cannot be used with scan targets\n")
			os.Exit(1)
		}

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
			if *exportPath != "-" {
				fmt.Printf("Exported to %s\n", *exportPath)
			}
			os.Exit(0)
		}

		app := ui.NewAppFromImport(*importPath)
		app.Version = version
		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := app.FatalError(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Build scan options
	opts := scanner.DefaultOptions()
	opts.ShowHidden = *showHidden
	if *noHidden {
		opts.ShowHidden = false
	}
	opts.DisableGC = *disableGC
	opts.FollowSymlinks = *followSymlinks
	if *concurrency < 0 {
		fmt.Fprintf(os.Stderr, "Error: concurrency (-j) must be >= 0\n")
		os.Exit(1)
	}
	opts.Concurrency = *concurrency

	if *exclude != "" {
		for _, e := range splitComma(*exclude) {
			if e != "" {
				opts.ExcludePatterns = append(opts.ExcludePatterns, e)
			}
		}
	}

	target, err := resolveScanTarget(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if target.Remote {
		if err := runRemoteScan(target, *sshPort, *sshBatch, *sshTimeout, *sshScanTimeout, *exportPath, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	absPath, err := filepath.Abs(target.LocalPath)
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

	// Headless export mode
	if *exportPath != "" {
		if *exportPath != "-" {
			fmt.Printf("Scanning %s...\n", absPath)
		}
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
		if *exportPath != "-" {
			fmt.Printf("Exported to %s\n", *exportPath)
		}
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
	if err := app.FatalError(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRemoteScan(target scanTarget, sshPort int, sshBatch bool, sshTimeout int, sshScanTimeout int, exportPath string, opts scanner.ScanOptions) error {
	cfg := remote.Config{
		Target:    target.SSHDestination,
		Port:      sshPort,
		BatchMode: sshBatch,
		Timeout:   time.Duration(sshTimeout) * time.Second,
	}
	if sshScanTimeout > 0 {
		cfg.ScanTimeout = time.Duration(sshScanTimeout) * time.Second
	}
	s := remote.NewSFTPScanner(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	progressCh := make(chan scanner.Progress, 10)

	var progressWg sync.WaitGroup
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		for p := range progressCh {
			fmt.Fprintf(os.Stderr, "\rScanning %s: %d files, %d dirs, %d errors...",
				target.SSHDestination, p.FilesScanned, p.DirsScanned, p.Errors)
		}
		fmt.Fprintln(os.Stderr)
	}()

	root, err := s.Scan(ctx, target.RemotePath, opts, progressCh)
	close(progressCh)
	progressWg.Wait()
	if err != nil {
		return err
	}

	if exportPath != "" {
		if err := ops.ExportJSON(root, exportPath, version); err != nil {
			return fmt.Errorf("export error: %w", err)
		}
		if exportPath != "-" {
			fmt.Printf("Exported to %s\n", exportPath)
		}
		return nil
	}

	tempFile, err := os.CreateTemp("", "godu-remote-*.json")
	if err != nil {
		return fmt.Errorf("cannot create temporary file for remote scan: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		return err
	}
	defer os.Remove(tempPath)

	exportErr := ops.ExportJSON(root, tempPath, version)
	if exportErr != nil {
		return fmt.Errorf("export error: %w", exportErr)
	}

	app := ui.NewAppFromImport(tempPath)
	app.Version = version
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	if err := app.FatalError(); err != nil {
		return err
	}
	return nil
}

func resolveScanTarget(args []string) (scanTarget, error) {
	if len(args) == 0 {
		return scanTarget{LocalPath: "."}, nil
	}

	first := args[0]
	if pathExists(first) {
		if len(args) > 1 {
			return scanTarget{}, fmt.Errorf("too many positional arguments for local scan")
		}
		return scanTarget{LocalPath: first}, nil
	}

	if isRemote, err := validateRemoteTarget(first); isRemote {
		if err != nil {
			return scanTarget{}, err
		}
		if len(args) > 2 {
			return scanTarget{}, fmt.Errorf("too many positional arguments for remote scan")
		}

		remotePath := "."
		if len(args) == 2 && strings.TrimSpace(args[1]) != "" {
			remotePath = args[1]
		}

		return scanTarget{
			Remote:         true,
			SSHDestination: first,
			RemotePath:     remotePath,
		}, nil
	}

	if len(args) > 1 {
		return scanTarget{}, fmt.Errorf("too many positional arguments")
	}

	return scanTarget{LocalPath: first}, nil
}

func validateRemoteTarget(raw string) (bool, error) {
	if strings.ContainsAny(raw, `/\\`) {
		return false, nil
	}
	if strings.Count(raw, "@") != 1 {
		return false, nil
	}

	user, host, _ := strings.Cut(raw, "@")
	if user == "" || host == "" {
		return true, fmt.Errorf("invalid remote target %q: expected user@host", raw)
	}
	if strings.HasPrefix(user, "-") || strings.HasPrefix(host, "-") {
		return true, fmt.Errorf("invalid remote target %q", raw)
	}
	if strings.ContainsAny(user, " \t\n\r") || strings.ContainsAny(host, " \t\n\r") {
		return true, fmt.Errorf("invalid remote target %q: spaces are not allowed", raw)
	}
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end == -1 {
			return true, fmt.Errorf("invalid remote target %q: malformed bracketed host", raw)
		}
		if end == 1 {
			return true, fmt.Errorf("invalid remote target %q: empty host", raw)
		}
		if end != len(host)-1 {
			rest := host[end+1:]
			if strings.HasPrefix(rest, ":") && isAllDigits(rest[1:]) {
				return true, fmt.Errorf("remote target %q must not include :port; use --ssh-port", raw)
			}
			return true, fmt.Errorf("invalid remote target %q: malformed bracketed host", raw)
		}
	} else if strings.Contains(host, "]") {
		return true, fmt.Errorf("invalid remote target %q: malformed bracketed host", raw)
	}
	if looksLikeHostPort(host) {
		return true, fmt.Errorf("remote target %q must not include :port; use --ssh-port", raw)
	}

	return true, nil
}

func looksLikeHostPort(host string) bool {
	if strings.Count(host, ":") != 1 {
		return false
	}
	_, port, ok := strings.Cut(host, ":")
	if !ok {
		return false
	}
	return isAllDigits(port)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
