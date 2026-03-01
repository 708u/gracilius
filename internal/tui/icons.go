package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	devicons "github.com/epilande/go-devicons"
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
	style  lipgloss.Style
}

var categoryStyles = map[fileCategory]categoryInfo{
	catSource: {"\u25c6", lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF"))},
	catConfig: {"\u25c7", lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5C07B"))},
	catDoc: {"\u25cb", lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98C379"))},
	catBinary: {"\u25a1", lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370"))},
	catOther: {"\u25cf", lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370"))},
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

func dirIcon(mode iconMode, entry fileEntry) string {
	if mode == iconNerd && entry.isDir {
		s := devicons.IconForPath(entry.path)
		st := lipgloss.NewStyle().
			Foreground(lipgloss.Color(s.Color))
		return st.Render(s.Icon) + " "
	}
	return ""
}

func fileIcon(mode iconMode, entry fileEntry) string {
	switch mode {
	case iconNerd:
		s := devicons.IconForPath(entry.path)
		st := lipgloss.NewStyle().
			Foreground(lipgloss.Color(s.Color))
		return st.Render(s.Icon) + " "
	default:
		cat := classifyFile(entry.name)
		ci := categoryStyles[cat]
		return ci.style.Render(ci.symbol) + " "
	}
}
