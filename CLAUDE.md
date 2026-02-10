# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Install

```bash
brew tap sadopc/tap && brew install godu   # install via Homebrew
go build -o godu ./cmd/godu               # build binary locally
go test ./...                              # run all tests
golangci-lint run ./...                    # lint
make release                               # cross-compile darwin/linux arm64/amd64
```

Version is injected at build time via `-ldflags "-X main.version=$(VERSION)"` using git describe.

Homebrew tap repo: `sadopc/homebrew-tap`. Formula at `Formula/godu.rb` — update SHA256 hashes and version when cutting a new release.

## Architecture

**Data flow:** `main.go` → `scanner.ParallelScanner` → `model.DirNode` tree → `ui.App` (Bubble Tea) → `components.*` renders

### Packages

- **`model`** — `FileNode`, `DirNode`, `TreeNode` interface. Paths are reconstructed by walking `Parent *DirNode` pointers (not stored per-node). Size vs Usage: apparent file size vs actual disk blocks (`stat.Blocks * 512`).
- **`scanner`** — Goroutine-per-directory with semaphore (`3 * GOMAXPROCS`). Progress via atomic counters, sent on a ticker channel every 50ms. Hardlink dedup via inode map. After `wg.Wait()`, `updateSizeRecursive()` does a single-threaded bottom-up size calculation — this ordering is critical.
- **`ui`** — Bubble Tea state machine with `AppState` (Scanning/Browsing/ConfirmDelete/Help/Exporting) and `ViewMode` (Tree/Treemap/FileType). Progress from scanner flows through a mutex-protected `latestProgress` snapshot read by `tickMsg` handler.
- **`ui/style`** — `Theme` struct (colors + lipgloss styles) and `Layout` (dimension math). `rowOverhead()` returns 23 — the fixed character width consumed by indicator, percentage, brackets, spacing, and size column in each tree row.
- **`ui/components`** — Stateless render functions. `TreeView` is a struct with `Render()` + `EnsureVisible()` for virtual scrolling. Others are pure functions (`RenderTreemap`, `RenderFileTypes`, `RenderHelp`, etc.).
- **`ops`** — Delete (os.RemoveAll), Export/Import (ncdu-compatible nested JSON arrays).

### Key patterns

- **Parent pointer paths:** `Path()` walks up parent chain and reverses. No full path strings stored on nodes.
- **Bottom-up size calc:** Scanner spawns parallel goroutines that build `Children` slices (mutex-protected). Sizes are only calculated after all goroutines finish via `updateSizeRecursive()`. Moving this into the parallel goroutines causes race conditions.
- **Progress snapshot:** Scanner goroutine writes to `latestProgress` under mutex. UI reads it on a 60ms tick. Not sent as Bubble Tea messages to avoid channel contention.
- **Row layout math:** Each tree row is: `indicator(2) + pct(6) + " ["(2) + bar(variable) + "] "(2) + name(variable) + " "(1) + size(10) = 23 fixed + bar + name`. `BarWidth` is clamped to [5, 40]. `NameWidth` gets the remainder.
- **Gradient bars:** Per-character color interpolation using `go-colorful` Lab color space blend (purple `#7B2FBE` → teal `#00D4AA`).

## Three execution modes

1. **Interactive TUI** (default): `godu /path` — scan with progress, browse tree
2. **Headless export**: `godu --export scan.json /path` — scan, write JSON, exit (no TUI)
3. **Import browse**: `godu --import scan.json` — load JSON, browse without rescanning
