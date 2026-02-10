package ops

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/serdar/godu/internal/model"
)

// ncdu-compatible JSON format:
// [1, 0, {"progname":"godu","progver":"1.0","timestamp":1234567890},
//   [{"name":"/path","asize":123,"dsize":456},
//     {"name":"file1","asize":10,"dsize":20},
//     [{"name":"subdir","asize":30,"dsize":40},
//       {"name":"file2","asize":5,"dsize":10}
//     ]
//   ]
// ]

type ncduHeader struct {
	Progname  string `json:"progname"`
	Progver   string `json:"progver"`
	Timestamp int64  `json:"timestamp"`
}

type ncduEntry struct {
	Name  string `json:"name"`
	Asize int64  `json:"asize"`
	Dsize int64  `json:"dsize,omitempty"`
	Ino   uint64 `json:"ino,omitempty"`
	Nlink int    `json:"nlink,omitempty"`
	Hlnkc bool   `json:"hlnkc,omitempty"`
	Err   bool   `json:"read_error,omitempty"`
}

// ExportJSON exports the tree to ncdu-compatible JSON format.
func ExportJSON(root *model.DirNode, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create export file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	// Write opening bracket and header
	f.WriteString("[1, 0, ")
	header := ncduHeader{
		Progname: "godu",
		Progver:  "1.0",
	}
	if err := enc.Encode(header); err != nil {
		return err
	}
	// Remove the trailing newline from Encode
	f.Seek(-1, 1)
	f.WriteString(",\n")

	// Write tree recursively
	if err := writeDir(f, root, true); err != nil {
		return err
	}

	f.WriteString("\n]\n")
	return nil
}

func writeDir(f *os.File, dir *model.DirNode, isRoot bool) error {
	// Open array for directory
	f.WriteString("[")

	// Directory entry
	entry := ncduEntry{
		Name:  dir.Name,
		Asize: dir.GetSize(),
		Dsize: dir.GetUsage(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f.Write(data)

	children := dir.GetChildren()
	for _, child := range children {
		f.WriteString(",\n")

		switch c := child.(type) {
		case *model.DirNode:
			if err := writeDir(f, c, false); err != nil {
				return err
			}
		case *model.FileNode:
			entry := ncduEntry{
				Name:  c.Name,
				Asize: c.Size,
				Dsize: c.Usage,
				Ino:   c.Inode,
			}
			if c.Flag&model.FlagHardlink != 0 {
				entry.Hlnkc = true
			}
			if c.Flag&model.FlagError != 0 {
				entry.Err = true
			}
			data, err := json.Marshal(entry)
			if err != nil {
				return err
			}
			f.Write(data)
		}
	}

	f.WriteString("]")
	return nil
}
