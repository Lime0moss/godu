package components

import (
	"sync"
	"testing"

	"github.com/sadopc/godu/internal/model"
	"github.com/sadopc/godu/internal/scanner"
	"github.com/sadopc/godu/internal/ui/style"
)

func TestRenderHelp_SmallWidth(t *testing.T) {
	theme := style.DefaultTheme()
	for _, w := range []int{0, 1, 2, 5} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderHelp panicked at width=%d: %v", w, r)
				}
			}()
			RenderHelp(theme, w, 10)
		})
	}
}

func TestRenderConfirmDialog_SmallWidth(t *testing.T) {
	theme := style.DefaultTheme()
	items := []ConfirmItem{{Name: "test.txt", Path: "/tmp/test.txt", Size: 100}}
	for _, w := range []int{0, 1, 2, 5} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderConfirmDialog panicked at width=%d: %v", w, r)
				}
			}()
			RenderConfirmDialog(theme, items, w, 10)
		})
	}
}

func TestRenderScanProgress_SmallWidth(t *testing.T) {
	theme := style.DefaultTheme()
	p := scanner.Progress{}
	for _, w := range []int{0, 1, 2, 5} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderScanProgress panicked at width=%d: %v", w, r)
				}
			}()
			RenderScanProgress(theme, p, w, 10)
		})
	}
}

func TestRenderFileTypes_SmallWidth(t *testing.T) {
	theme := style.DefaultTheme()
	dir := &model.DirNode{
		FileNode: model.FileNode{Name: "root"},
	}
	for _, w := range []int{0, 1, 2, 5} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderFileTypes panicked at width=%d: %v", w, r)
				}
			}()
			RenderFileTypes(theme, dir, false, true, w, 10)
		})
	}
}

func TestRenderFileTypes_ConcurrentCacheAccess(t *testing.T) {
	theme := style.DefaultTheme()
	root := &model.DirNode{FileNode: model.FileNode{Name: "root"}}
	root.AddChild(&model.FileNode{Name: "a.txt", Size: 10, Usage: 10, Parent: root})
	root.UpdateSizeRecursive()

	const workers = 8
	const iterations = 40
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if j%5 == 0 {
					InvalidateFileTypeCache()
				}
				RenderFileTypes(theme, root, j%2 == 0, true, 80, 20)
			}
		}(i)
	}

	wg.Wait()
}
