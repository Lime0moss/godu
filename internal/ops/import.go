package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sadopc/godu/internal/model"
)

// validateName rejects names that could escape the directory tree.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("empty entry name")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid entry name: %q", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("entry name contains path separator: %q", name)
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("entry name is not a simple filename: %q", name)
	}
	return nil
}

// ImportJSON imports a tree from ncdu-compatible JSON format.
func ImportJSON(path string) (*model.DirNode, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open import file: %w", err)
	}
	defer f.Close()

	// Parse the top-level array: [version, minor, header, rootDir]
	var raw []json.RawMessage
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if len(raw) < 4 {
		return nil, fmt.Errorf("invalid ncdu format: expected at least 4 elements, got %d", len(raw))
	}

	// raw[3] is the root directory array
	root, err := parseDir(raw[3], nil)
	if err != nil {
		return nil, fmt.Errorf("cannot parse root directory: %w", err)
	}

	root.UpdateSize()
	return root, nil
}

func parseDir(data json.RawMessage, parent *model.DirNode) (*model.DirNode, error) {
	// A directory is an array: [{dir_entry}, child1, child2, ...]
	var elements []json.RawMessage
	if err := json.Unmarshal(data, &elements); err != nil {
		return nil, fmt.Errorf("directory is not an array: %w", err)
	}

	if len(elements) == 0 {
		return nil, fmt.Errorf("empty directory array")
	}

	// First element is the directory entry object
	var entry ncduEntry
	if err := json.Unmarshal(elements[0], &entry); err != nil {
		return nil, fmt.Errorf("cannot parse directory entry: %w", err)
	}

	// Root entry (parent==nil) uses an absolute path per ncdu convention;
	// non-root entries must be simple filenames.
	if parent != nil {
		if err := validateName(entry.Name); err != nil {
			return nil, fmt.Errorf("invalid directory entry: %w", err)
		}
	} else {
		entry.Name = filepath.Clean(entry.Name)
	}

	dir := &model.DirNode{
		FileNode: model.FileNode{
			Name:   entry.Name,
			Size:   entry.Asize,
			Usage:  entry.Dsize,
			Mtime:  time.Time{},
			Parent: parent,
		},
	}

	// Remaining elements are children (objects = files, arrays = subdirs)
	for i := 1; i < len(elements); i++ {
		child := elements[i]

		// Check if it starts with '[' (directory) or '{' (file)
		trimmed := trimLeadingWhitespace(child)
		if len(trimmed) == 0 {
			continue
		}

		if trimmed[0] == '[' {
			// Subdirectory
			subDir, err := parseDir(child, dir)
			if err != nil {
				return nil, err
			}
			dir.AddChild(subDir)
		} else if trimmed[0] == '{' {
			// File entry
			var fileEntry ncduEntry
			if err := json.Unmarshal(child, &fileEntry); err != nil {
				return nil, fmt.Errorf("cannot parse file entry: %w", err)
			}

			if err := validateName(fileEntry.Name); err != nil {
				return nil, fmt.Errorf("invalid file entry: %w", err)
			}

			var flag model.NodeFlag
			if fileEntry.Hlnkc {
				flag |= model.FlagHardlink
			}
			if fileEntry.Err {
				flag |= model.FlagError
			}
			if fileEntry.Symlink {
				flag |= model.FlagSymlink
			}

			fileNode := &model.FileNode{
				Name:   fileEntry.Name,
				Size:   fileEntry.Asize,
				Usage:  fileEntry.Dsize,
				Inode:  fileEntry.Ino,
				Flag:   flag,
				Parent: dir,
			}
			dir.AddChild(fileNode)
		} else {
			return nil, fmt.Errorf("unexpected child element at index %d: expected array or object", i)
		}
	}

	dir.UpdateSize()
	return dir, nil
}

func trimLeadingWhitespace(data []byte) []byte {
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return data[i:]
		}
	}
	return nil
}
