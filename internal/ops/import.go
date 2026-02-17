package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	if strings.ContainsRune(name, '/') {
		return fmt.Errorf("entry name contains path separator: %q", name)
	}
	if runtime.GOOS == "windows" && strings.ContainsRune(name, '\\') {
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

	dec := json.NewDecoder(f)
	if err := expectDelim(dec, '[', "invalid JSON: top-level value must be an array"); err != nil {
		return nil, err
	}

	var root *model.DirNode
	elem := 0
	for dec.More() {
		switch elem {
		case 0, 1, 2:
			var discard any
			if err := dec.Decode(&discard); err != nil {
				return nil, fmt.Errorf("invalid ncdu format: cannot parse top-level element %d: %w", elem, err)
			}
		case 3:
			subdir, err := parseDirFromDecoder(dec, nil, 0, false)
			if err != nil {
				return nil, fmt.Errorf("cannot parse root directory: %w", err)
			}
			root = subdir
		default:
			// Ignore optional trailing top-level metadata while still validating JSON.
			var discard any
			if err := dec.Decode(&discard); err != nil {
				return nil, fmt.Errorf("invalid ncdu format: cannot parse top-level element %d: %w", elem, err)
			}
		}
		elem++
	}
	if err := expectDelim(dec, ']', "invalid JSON: malformed top-level array"); err != nil {
		return nil, err
	}

	if elem < 4 {
		return nil, fmt.Errorf("invalid ncdu format: expected at least 4 elements, got %d", elem)
	}
	if root == nil {
		return nil, fmt.Errorf("invalid ncdu format: missing root directory")
	}

	// Reject trailing non-whitespace input.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("invalid JSON: trailing data after top-level array")
		}
		return nil, fmt.Errorf("invalid JSON: trailing data after top-level array: %w", err)
	}

	root.UpdateSize()
	return root, nil
}

const maxImportDepth = 1000

func parseDirFromDecoder(dec *json.Decoder, parent *model.DirNode, depth int, openConsumed bool) (*model.DirNode, error) {
	if depth > maxImportDepth {
		return nil, fmt.Errorf("directory nesting exceeds maximum depth of %d", maxImportDepth)
	}
	if !openConsumed {
		if err := expectDelim(dec, '[', "directory is not an array"); err != nil {
			return nil, err
		}
	}
	if !dec.More() {
		return nil, fmt.Errorf("empty directory array")
	}

	// First element is the directory entry object.
	entry, err := parseNCDUEntry(dec, false)
	if err != nil {
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

	var dirFlag model.NodeFlag
	if entry.Hlnkc {
		dirFlag |= model.FlagHardlink
	}
	if entry.Err {
		dirFlag |= model.FlagError
	}
	if entry.Symlink {
		dirFlag |= model.FlagSymlink
	}
	if entry.UsageEstimated {
		dirFlag |= model.FlagUsageEstimated
	}
	if err := validateSizeField("directory asize", entry.Asize); err != nil {
		return nil, err
	}
	if err := validateSizeField("directory dsize", entry.Dsize); err != nil {
		return nil, err
	}

	dir := &model.DirNode{
		FileNode: model.FileNode{
			Name:   entry.Name,
			Size:   entry.Asize,
			Usage:  entry.Dsize,
			Mtime:  time.Time{},
			Flag:   dirFlag,
			Parent: parent,
		},
	}

	// Remaining elements are children (objects = files, arrays = subdirs).
	for i := 1; dec.More(); i++ {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot parse child at index %d: %w", i, err)
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil, fmt.Errorf("unexpected child element at index %d: expected array or object", i)
		}

		switch delim {
		case '[':
			subDir, err := parseDirFromDecoder(dec, dir, depth+1, true)
			if err != nil {
				return nil, err
			}
			dir.AddChild(subDir)
		case '{':
			fileEntry, err := parseNCDUEntry(dec, true)
			if err != nil {
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
			if fileEntry.UsageEstimated {
				flag |= model.FlagUsageEstimated
			}
			if err := validateSizeField("file asize", fileEntry.Asize); err != nil {
				return nil, err
			}
			if err := validateSizeField("file dsize", fileEntry.Dsize); err != nil {
				return nil, err
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
		default:
			return nil, fmt.Errorf("unexpected child element at index %d: expected array or object", i)
		}
	}
	if err := expectDelim(dec, ']', "directory is not an array"); err != nil {
		return nil, err
	}

	dir.UpdateSize()
	return dir, nil
}

func parseNCDUEntry(dec *json.Decoder, openConsumed bool) (ncduEntry, error) {
	if !openConsumed {
		if err := expectDelim(dec, '{', "entry is not an object"); err != nil {
			return ncduEntry{}, err
		}
	}

	var entry ncduEntry
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return ncduEntry{}, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return ncduEntry{}, fmt.Errorf("entry object has non-string key")
		}

		switch key {
		case "name":
			if err := dec.Decode(&entry.Name); err != nil {
				return ncduEntry{}, err
			}
		case "asize":
			if err := dec.Decode(&entry.Asize); err != nil {
				return ncduEntry{}, err
			}
		case "dsize":
			if err := dec.Decode(&entry.Dsize); err != nil {
				return ncduEntry{}, err
			}
		case "ino":
			if err := dec.Decode(&entry.Ino); err != nil {
				return ncduEntry{}, err
			}
		case "nlink":
			if err := dec.Decode(&entry.Nlink); err != nil {
				return ncduEntry{}, err
			}
		case "hlnkc":
			if err := dec.Decode(&entry.Hlnkc); err != nil {
				return ncduEntry{}, err
			}
		case "read_error":
			if err := dec.Decode(&entry.Err); err != nil {
				return ncduEntry{}, err
			}
		case "symlink":
			if err := dec.Decode(&entry.Symlink); err != nil {
				return ncduEntry{}, err
			}
		case "usage_estimated":
			if err := dec.Decode(&entry.UsageEstimated); err != nil {
				return ncduEntry{}, err
			}
		default:
			// Unknown fields are ignored for forward compatibility.
			var discard any
			if err := dec.Decode(&discard); err != nil {
				return ncduEntry{}, err
			}
		}
	}

	if err := expectDelim(dec, '}', "entry is not an object"); err != nil {
		return ncduEntry{}, err
	}
	return entry, nil
}

func expectDelim(dec *json.Decoder, want rune, msg string) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	d, ok := tok.(json.Delim)
	if !ok || rune(d) != want {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func validateSizeField(field string, value int64) error {
	if value < 0 {
		return fmt.Errorf("%s must be non-negative", field)
	}
	return nil
}
