package util

import "strings"

// Icon returns a Unicode icon for the given filename or directory.
func Icon(name string, isDir bool) string {
	if isDir {
		return DirIcon(name)
	}
	return FileIcon(name)
}

// DirIcon returns an icon for a directory name.
func DirIcon(name string) string {
	lower := strings.ToLower(name)
	if icon, ok := dirIcons[lower]; ok {
		return icon
	}
	return "📁"
}

// FileIcon returns an icon based on file extension.
func FileIcon(name string) string {
	ext := strings.ToLower(getExt(name))
	if icon, ok := extIcons[ext]; ok {
		return icon
	}
	return "📄"
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

var dirIcons = map[string]string{
	".git":         "🔀",
	"node_modules": "📦",
	"vendor":       "📦",
	"dist":         "📤",
	"build":        "🔨",
	"target":       "🎯",
	"src":          "💻",
	"lib":          "📚",
	"test":         "🧪",
	"tests":        "🧪",
	"docs":         "📝",
	"doc":          "📝",
	"config":       "⚙️",
	"bin":          "⚡",
	"tmp":          "🕐",
	"cache":        "💾",
	".cache":       "💾",
	"assets":       "🎨",
	"public":       "🌐",
	"static":       "🌐",
	"images":       "🖼️",
	"img":          "🖼️",
}

var extIcons = map[string]string{
	// Code
	".go":     "🐹",
	".py":     "🐍",
	".js":     "🟨",
	".ts":     "🔷",
	".jsx":    "⚛️",
	".tsx":    "⚛️",
	".rs":     "🦀",
	".c":      "🔵",
	".cpp":    "🔵",
	".java":   "☕",
	".rb":     "💎",
	".swift":  "🐦",
	".kt":     "🟣",
	".php":    "🐘",
	".html":   "🌐",
	".css":    "🎨",
	".scss":   "🎨",
	".vue":    "💚",
	".svelte": "🔥",

	// Data
	".json": "📋",
	".yaml": "📋",
	".yml":  "📋",
	".toml": "📋",
	".xml":  "📋",
	".csv":  "📊",
	".sql":  "🗃️",

	// Documents
	".md":   "📝",
	".txt":  "📄",
	".pdf":  "📕",
	".doc":  "📘",
	".docx": "📘",
	".xls":  "📗",
	".xlsx": "📗",

	// Media
	".mp4":  "🎬",
	".mkv":  "🎬",
	".avi":  "🎬",
	".mov":  "🎬",
	".mp3":  "🎵",
	".flac": "🎵",
	".wav":  "🎵",
	".ogg":  "🎵",
	".jpg":  "🖼️",
	".jpeg": "🖼️",
	".png":  "🖼️",
	".gif":  "🖼️",
	".svg":  "🖼️",
	".webp": "🖼️",

	// Archives
	".zip": "📦",
	".tar": "📦",
	".gz":  "📦",
	".rar": "📦",
	".7z":  "📦",
	".iso": "💿",
	".dmg": "💿",

	// System
	".log":  "📜",
	".lock": "🔒",
	".env":  "🔐",
	".db":   "🗄️",

	// Executables
	".exe":  "⚡",
	".bin":  "⚡",
	".sh":   "🐚",
	".bash": "🐚",
	".zsh":  "🐚",
}
