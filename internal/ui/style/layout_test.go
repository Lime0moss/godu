package style

import (
	"testing"
)

func TestContentHeight(t *testing.T) {
	tests := []struct {
		w, h int
		want int
	}{
		{80, 24, 20},
		{10, 5, 1},
		{10, 4, 1},  // 4-4=0, clamped to 1
		{10, 0, 1},  // negative, clamped to 1
		{80, 50, 46},
	}

	for _, tt := range tests {
		l := NewLayout(tt.w, tt.h)
		got := l.ContentHeight()
		if got != tt.want {
			t.Errorf("NewLayout(%d,%d).ContentHeight() = %d, want %d", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestBarWidth(t *testing.T) {
	tests := []struct {
		width int
		want  int
	}{
		{10, 5},   // 10-23 = negative, clamped to 5
		{30, 7},   // 30-23 = 7
		{80, 40},  // 80-23 = 57, clamped to 40
		{200, 40}, // clamped to 40
	}

	for _, tt := range tests {
		l := NewLayout(tt.width, 24)
		got := l.BarWidth()
		if got != tt.want {
			t.Errorf("NewLayout(%d,24).BarWidth() = %d, want %d", tt.width, got, tt.want)
		}
	}
}

func TestNameWidth(t *testing.T) {
	tests := []struct {
		width int
	}{
		{10},
		{30},
		{80},
		{200},
	}

	for _, tt := range tests {
		l := NewLayout(tt.width, 24)
		got := l.NameWidth()
		if got < 1 {
			t.Errorf("NewLayout(%d,24).NameWidth() = %d, want >= 1", tt.width, got)
		}
	}

	// For a wide terminal, NameWidth + BarWidth + overhead = ContentWidth
	l := NewLayout(80, 24)
	total := l.NameWidth() + l.BarWidth() + l.rowOverhead()
	if total != l.ContentWidth() {
		t.Errorf("NameWidth(%d) + BarWidth(%d) + overhead(%d) = %d, want ContentWidth %d",
			l.NameWidth(), l.BarWidth(), l.rowOverhead(), total, l.ContentWidth())
	}
}

func TestFullWidth(t *testing.T) {
	// Shorter than target — should be padded
	got := FullWidth("hi", 5)
	if len(got) != 5 {
		t.Errorf("FullWidth(\"hi\", 5) len = %d, want 5", len(got))
	}
	if got != "hi   " {
		t.Errorf("FullWidth(\"hi\", 5) = %q, want %q", got, "hi   ")
	}

	// Exact width — no change
	got = FullWidth("hello", 5)
	if got != "hello" {
		t.Errorf("FullWidth(\"hello\", 5) = %q, want %q", got, "hello")
	}
}
