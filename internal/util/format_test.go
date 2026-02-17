package util

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
		{1125899906842624, "1.0 PiB"},
		{-1, "0 B"},
		{-100, "0 B"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
		{1000000000, "1.0B"},
		{2000000000, "2.0B"},
	}

	for _, tt := range tests {
		got := FormatCount(tt.n)
		if got != tt.want {
			t.Errorf("FormatCount(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		part, total int64
		want        float64
	}{
		{0, 0, 0},
		{50, 100, 50},
		{100, 100, 100},
		{150, 100, 150},
		{1, 3, 100.0 / 3.0},
	}

	for _, tt := range tests {
		got := Percent(tt.part, tt.total)
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.001 {
			t.Errorf("Percent(%d, %d) = %f, want %f", tt.part, tt.total, got, tt.want)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		s      string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "he..."},
		{"hello", 3, "hel"},
		{"hello", 2, "he"},
		{"hello", 1, "h"},
		{"hello", 0, ""},
		{"こんにちは", 3, "こんに"},
		{"こんにちは", 5, "こんにちは"},
		{"abcdefgh", 6, "abc..."},
	}

	for _, tt := range tests {
		got := TruncateString(tt.s, tt.maxLen)
		if got != tt.want {
			t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
		}
	}
}
