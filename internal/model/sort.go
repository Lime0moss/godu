package model

import (
	"sort"
	"strings"

	"github.com/maruel/natural"
)

// SortField defines what to sort by.
type SortField int

const (
	SortBySize  SortField = iota
	SortByName
	SortByCount
	SortByMtime
)

// SortOrder defines ascending or descending.
type SortOrder int

const (
	SortDesc SortOrder = iota
	SortAsc
)

// SortConfig holds sort preferences.
type SortConfig struct {
	Field SortField
	Order SortOrder
	// DirsFirst keeps directories before files regardless of sort.
	DirsFirst bool
}

// DefaultSort returns the default sort config (size descending, dirs first).
func DefaultSort() SortConfig {
	return SortConfig{
		Field:     SortBySize,
		Order:     SortDesc,
		DirsFirst: true,
	}
}

// SortChildren sorts a slice of TreeNode in place according to config.
func SortChildren(children []TreeNode, cfg SortConfig, useApparent bool) {
	sort.SliceStable(children, func(i, j int) bool {
		a, b := children[i], children[j]

		// Dirs first
		if cfg.DirsFirst {
			aDir, bDir := a.IsDir(), b.IsDir()
			if aDir != bDir {
				return aDir
			}
		}

		// For descending order, swap a and b so the same less-than
		// comparisons produce the reverse result. This preserves
		// strict weak ordering (equal items return false, not true).
		if cfg.Order == SortDesc {
			a, b = b, a
		}

		var less bool
		switch cfg.Field {
		case SortBySize:
			var sa, sb int64
			if useApparent {
				sa, sb = a.GetSize(), b.GetSize()
			} else {
				sa, sb = a.GetUsage(), b.GetUsage()
			}
			less = sa < sb
		case SortByName:
			less = natural.Less(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
		case SortByCount:
			ca, cb := int64(1), int64(1)
			if da, ok := a.(*DirNode); ok {
				ca = da.ItemCount
			}
			if db, ok := b.(*DirNode); ok {
				cb = db.ItemCount
			}
			less = ca < cb
		case SortByMtime:
			less = a.GetMtime().Before(b.GetMtime())
		default:
			less = a.GetSize() < b.GetSize()
		}

		return less
	})
}
