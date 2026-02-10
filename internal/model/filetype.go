package model

import "strings"

// FileCategory represents a high-level file type category.
type FileCategory int

const (
	CatOther FileCategory = iota
	CatMedia
	CatCode
	CatArchive
	CatDocument
	CatSystem
	CatExecutable
)

// CategoryName returns the display name for a category.
func CategoryName(cat FileCategory) string {
	switch cat {
	case CatMedia:
		return "Media"
	case CatCode:
		return "Code"
	case CatArchive:
		return "Archives"
	case CatDocument:
		return "Documents"
	case CatSystem:
		return "System"
	case CatExecutable:
		return "Executables"
	default:
		return "Other"
	}
}

// CategoryColor returns the theme color for a category.
func CategoryColor(cat FileCategory) string {
	switch cat {
	case CatMedia:
		return "#E06C75" // Red
	case CatCode:
		return "#61AFEF" // Blue
	case CatArchive:
		return "#E5C07B" // Yellow
	case CatDocument:
		return "#98C379" // Green
	case CatSystem:
		return "#C678DD" // Purple
	case CatExecutable:
		return "#D19A66" // Orange
	default:
		return "#ABB2BF" // Gray
	}
}

// extMap maps file extensions to categories.
var extMap = map[string]FileCategory{
	// Media - Images
	".jpg": CatMedia, ".jpeg": CatMedia, ".png": CatMedia, ".gif": CatMedia,
	".bmp": CatMedia, ".svg": CatMedia, ".webp": CatMedia, ".ico": CatMedia,
	".tiff": CatMedia, ".tif": CatMedia, ".psd": CatMedia, ".raw": CatMedia,
	".cr2": CatMedia, ".nef": CatMedia, ".heic": CatMedia, ".heif": CatMedia,
	".avif": CatMedia, ".jxl": CatMedia,
	// Media - Video
	".mp4": CatMedia, ".mkv": CatMedia, ".avi": CatMedia, ".mov": CatMedia,
	".wmv": CatMedia, ".flv": CatMedia, ".webm": CatMedia, ".m4v": CatMedia,
	".mpg": CatMedia, ".mpeg": CatMedia, ".3gp": CatMedia, ".mts": CatMedia,
	// Media - Audio
	".mp3": CatMedia, ".flac": CatMedia, ".wav": CatMedia, ".aac": CatMedia,
	".ogg": CatMedia, ".wma": CatMedia, ".m4a": CatMedia, ".opus": CatMedia,
	".aiff": CatMedia, ".mid": CatMedia, ".midi": CatMedia,

	// Code
	".go": CatCode, ".py": CatCode, ".js": CatCode, ".jsx": CatCode,
	".ts": CatCode, ".tsx": CatCode, ".rs": CatCode, ".c": CatCode,
	".cpp": CatCode, ".cc": CatCode, ".h": CatCode, ".hpp": CatCode,
	".java": CatCode, ".kt": CatCode, ".swift": CatCode, ".rb": CatCode,
	".php": CatCode, ".cs": CatCode, ".scala": CatCode, ".clj": CatCode,
	".ex": CatCode, ".exs": CatCode, ".erl": CatCode, ".hs": CatCode,
	".ml": CatCode, ".lua": CatCode, ".r": CatCode, ".R": CatCode,
	".dart": CatCode, ".vue": CatCode, ".svelte": CatCode,
	".html": CatCode, ".htm": CatCode, ".css": CatCode, ".scss": CatCode,
	".sass": CatCode, ".less": CatCode, ".sql": CatCode, ".sh": CatCode,
	".bash": CatCode, ".zsh": CatCode, ".fish": CatCode, ".ps1": CatCode,
	".bat": CatCode, ".cmd": CatCode, ".zig": CatCode, ".nim": CatCode,
	".v": CatCode, ".asm": CatCode, ".s": CatCode, ".pl": CatCode,
	".pm": CatCode, ".tcl": CatCode, ".groovy": CatCode, ".gradle": CatCode,

	// Archives
	".zip": CatArchive, ".tar": CatArchive, ".gz": CatArchive, ".bz2": CatArchive,
	".xz": CatArchive, ".zst": CatArchive, ".lz4": CatArchive, ".lzma": CatArchive,
	".rar": CatArchive, ".7z": CatArchive, ".cab": CatArchive, ".iso": CatArchive,
	".dmg": CatArchive, ".pkg": CatArchive, ".deb": CatArchive, ".rpm": CatArchive,
	".snap": CatArchive, ".flatpak": CatArchive, ".appimage": CatArchive,
	".tgz": CatArchive, ".tbz2": CatArchive, ".txz": CatArchive,
	".jar": CatArchive, ".war": CatArchive, ".ear": CatArchive,

	// Documents
	".pdf": CatDocument, ".doc": CatDocument, ".docx": CatDocument,
	".xls": CatDocument, ".xlsx": CatDocument, ".ppt": CatDocument,
	".pptx": CatDocument, ".odt": CatDocument, ".ods": CatDocument,
	".odp": CatDocument, ".rtf": CatDocument, ".txt": CatDocument,
	".md": CatDocument, ".rst": CatDocument, ".tex": CatDocument,
	".csv": CatDocument, ".tsv": CatDocument, ".epub": CatDocument,
	".mobi": CatDocument, ".djvu": CatDocument, ".pages": CatDocument,
	".numbers": CatDocument, ".key": CatDocument,

	// System
	".log": CatSystem, ".bak": CatSystem, ".tmp": CatSystem, ".temp": CatSystem,
	".swp": CatSystem, ".swo": CatSystem, ".pid": CatSystem, ".lock": CatSystem,
	".cache": CatSystem, ".sock": CatSystem, ".dat": CatSystem, ".db": CatSystem,
	".sqlite": CatSystem, ".sqlite3": CatSystem, ".ldb": CatSystem,
	".plist": CatSystem, ".ini": CatSystem, ".cfg": CatSystem, ".conf": CatSystem,
	".sys": CatSystem, ".dll": CatSystem, ".dylib": CatSystem, ".so": CatSystem,

	// Executables
	".exe": CatExecutable, ".app": CatExecutable, ".msi": CatExecutable,
	".bin": CatExecutable, ".elf": CatExecutable, ".out": CatExecutable,
	".wasm": CatExecutable, ".pyc": CatExecutable, ".pyo": CatExecutable,
	".class": CatExecutable, ".o": CatExecutable, ".a": CatExecutable,

	// Config files (Code)
	".json": CatCode, ".yaml": CatCode, ".yml": CatCode, ".toml": CatCode,
	".xml": CatCode, ".proto": CatCode, ".graphql": CatCode, ".gql": CatCode,
	".env": CatCode, ".editorconfig": CatCode, ".gitignore": CatCode,
	".dockerignore": CatCode, ".makefile": CatCode,
}

// ClassifyFile returns the category for a given filename.
func ClassifyFile(name string) FileCategory {
	ext := strings.ToLower(getExt(name))
	if cat, ok := extMap[ext]; ok {
		return cat
	}
	return CatOther
}

// GetExtension returns the lowercase extension of a filename.
func GetExtension(name string) string {
	return strings.ToLower(getExt(name))
}

func getExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i:]
		}
		if name[i] == '/' {
			break
		}
	}
	return ""
}
