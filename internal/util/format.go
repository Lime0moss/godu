package util

import "fmt"

// FormatSize returns a human-readable size string.
func FormatSize(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}

	const (
		_          = iota
		kB float64 = 1 << (10 * iota)
		mB
		gB
		tB
		pB
	)

	b := float64(bytes)
	switch {
	case b >= pB:
		return fmt.Sprintf("%.1f PiB", b/pB)
	case b >= tB:
		return fmt.Sprintf("%.1f TiB", b/tB)
	case b >= gB:
		return fmt.Sprintf("%.1f GiB", b/gB)
	case b >= mB:
		return fmt.Sprintf("%.1f MiB", b/mB)
	case b >= kB:
		return fmt.Sprintf("%.1f KiB", b/kB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatCount returns a human-readable count string.
func FormatCount(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	if n < 1_000_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
}

// Percent returns the percentage of part relative to total.
func Percent(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// TruncateString truncates a string to maxLen runes, adding "..." if needed.
func TruncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
