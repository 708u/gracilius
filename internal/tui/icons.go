package tui

import (
	"os"
	"path/filepath"
	"strings"

	devicons "github.com/epilande/go-devicons"
	"github.com/muesli/termenv"
)

type iconMode int

const (
	iconSymbol iconMode = iota
	iconNerd
)

func detectIconMode() iconMode {
	if strings.EqualFold(os.Getenv("GRA_ICONS"), "nerd") {
		return iconNerd
	}
	return iconSymbol
}

// --- symbol mode ---

type fileCategory int

const (
	catSource fileCategory = iota
	catConfig
	catDoc
	catBinary
	catOther
)

type categoryInfo struct {
	symbol string
	color  string
}

var categoryStyles = map[fileCategory]categoryInfo{
	catSource: {"\u25c6", "#61AFEF"},
	catConfig: {"\u25c7", "#E5C07B"},
	catDoc:    {"\u25cb", "#98C379"},
	catBinary: {"\u25a1", "#5C6370"},
	catOther:  {"\u25cf", "#5C6370"},
}

var extCategory = map[string]fileCategory{
	// Source code
	".go":    catSource,
	".js":    catSource,
	".ts":    catSource,
	".tsx":   catSource,
	".jsx":   catSource,
	".py":    catSource,
	".rs":    catSource,
	".java":  catSource,
	".c":     catSource,
	".cpp":   catSource,
	".h":     catSource,
	".hpp":   catSource,
	".rb":    catSource,
	".sh":    catSource,
	".lua":   catSource,
	".swift": catSource,
	".kt":    catSource,
	".cs":    catSource,
	".scala": catSource,
	".ex":    catSource,
	".exs":   catSource,
	".zig":   catSource,
	".hs":    catSource,
	".ml":    catSource,
	".clj":   catSource,
	".php":   catSource,
	".pl":    catSource,
	".r":     catSource,
	".dart":  catSource,
	".v":     catSource,
	".nim":   catSource,
	".cr":    catSource,
	".jl":    catSource,
	".elm":   catSource,
	".erl":   catSource,
	".gleam": catSource,

	// Config / dependency
	".json":       catConfig,
	".yaml":       catConfig,
	".yml":        catConfig,
	".toml":       catConfig,
	".xml":        catConfig,
	".ini":        catConfig,
	".env":        catConfig,
	".mod":        catConfig,
	".sum":        catConfig,
	".lock":       catConfig,
	".cfg":        catConfig,
	".conf":       catConfig,
	".properties": catConfig,

	// Documentation
	".md":   catDoc,
	".txt":  catDoc,
	".rst":  catDoc,
	".adoc": catDoc,

	// Binary / media
	".png":   catBinary,
	".jpg":   catBinary,
	".jpeg":  catBinary,
	".gif":   catBinary,
	".svg":   catBinary,
	".ico":   catBinary,
	".wasm":  catBinary,
	".pdf":   catBinary,
	".zip":   catBinary,
	".tar":   catBinary,
	".gz":    catBinary,
	".bz2":   catBinary,
	".7z":    catBinary,
	".mp3":   catBinary,
	".mp4":   catBinary,
	".wav":   catBinary,
	".webm":  catBinary,
	".webp":  catBinary,
	".ttf":   catBinary,
	".woff":  catBinary,
	".woff2": catBinary,
	".eot":   catBinary,
}

var nameCategory = map[string]fileCategory{
	"makefile":      catConfig,
	"dockerfile":    catConfig,
	".gitignore":    catConfig,
	".editorconfig": catConfig,
	".prettierrc":   catConfig,
	".eslintrc":     catConfig,
	"license":       catDoc,
	"readme":        catDoc,
	"changelog":     catDoc,
}

func classifyFile(name string) fileCategory {
	lower := strings.ToLower(name)
	if cat, ok := nameCategory[lower]; ok {
		return cat
	}
	ext := strings.ToLower(filepath.Ext(name))
	if cat, ok := extCategory[ext]; ok {
		return cat
	}
	return catOther
}

// iconInfo holds the raw icon character and its foreground color.
type iconInfo struct {
	char  string
	color string
}

// iconInfoFor returns icon info for the given entry, or nil if
// no icon should be displayed (symbol-mode directories).
func iconInfoFor(mode iconMode, entry fileEntry) *iconInfo {
	if mode == iconNerd {
		s := devicons.IconForPath(entry.path)
		return &iconInfo{char: s.Icon, color: s.Color}
	}
	if entry.isDir {
		return nil
	}
	cat := classifyFile(entry.name)
	ci := categoryStyles[cat]
	return &iconInfo{char: ci.symbol, color: ci.color}
}

const ansiFgReset = termenv.CSI + "39m"

// colorizeIcon injects ANSI foreground color around the icon position
// in a line. pos is the byte offset of the icon character.
// This preserves any existing ANSI (e.g. background) on the line.
func colorizeIcon(line string, pos int, info iconInfo) string {
	set := termenv.CSI +
		termenv.RGBColor(info.color).Sequence(false) + "m"
	iconEnd := pos + len(info.char)
	return line[:pos] + set + line[pos:iconEnd] + ansiFgReset + line[iconEnd:]
}
