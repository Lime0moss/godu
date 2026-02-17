package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/sadopc/godu/internal/model"
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

// errWriter wraps an *os.File and captures the first write/seek error.
// Subsequent writes after an error are no-ops, avoiding verbose per-call checks.
type errWriter struct {
	w   *os.File
	err error
}

func (ew *errWriter) WriteString(s string) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.WriteString(s)
}

func (ew *errWriter) Write(data []byte) (int, error) {
	if ew.err != nil {
		return 0, ew.err
	}
	n, err := ew.w.Write(data)
	if err != nil {
		ew.err = err
	}
	return n, err
}

func (ew *errWriter) seek(offset int64, whence int) {
	if ew.err != nil {
		return
	}
	_, ew.err = ew.w.Seek(offset, whence)
}

// ExportJSON exports the tree to ncdu-compatible JSON format.
func ExportJSON(root *model.DirNode, path string, version string) (retErr error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("cannot create export file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	ew := &errWriter{w: f}

	// Write opening bracket and header
	ew.WriteString("[1, 0, ")
	if version == "" {
		version = "dev"
	}
	header := ncduHeader{
		Progname:  "godu",
		Progver:   version,
		Timestamp: time.Now().Unix(),
	}
	enc := json.NewEncoder(ew)
	if err := enc.Encode(header); err != nil {
		return err
	}
	// Remove the trailing newline from Encode
	ew.seek(-1, io.SeekCurrent)
	ew.WriteString(",\n")

	// Write tree recursively
	writeDir(ew, root)

	ew.WriteString("\n]\n")
	return ew.err
}

func writeDir(ew *errWriter, dir *model.DirNode) {
	if ew.err != nil {
		return
	}

	// Open array for directory
	ew.WriteString("[")

	// Directory entry
	entry := ncduEntry{
		Name:  dir.Name,
		Asize: dir.GetSize(),
		Dsize: dir.GetUsage(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		ew.err = err
		return
	}
	ew.Write(data)

	children := dir.GetChildren()
	for _, child := range children {
		if ew.err != nil {
			return
		}
		ew.WriteString(",\n")

		switch c := child.(type) {
		case *model.DirNode:
			writeDir(ew, c)
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
				ew.err = err
				return
			}
			ew.Write(data)
		}
	}

	ew.WriteString("]")
}
