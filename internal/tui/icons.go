package tui

import (
	"os"
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
	if strings.EqualFold(os.Getenv("GRA_ICONS"), "symbol") {
		return iconSymbol
	}
	return iconNerd
}

// --- symbol mode ---

var (
	symbolText   = iconInfo{"\u25c6", "#61AFEF"}
	symbolBinary = iconInfo{"\u25a1", "#5C6370"}
)

// iconInfo holds the raw icon character and its foreground color.
type iconInfo struct {
	char  string
	color string
}

// iconFor returns icon info for the given entry, or nil if
// no icon should be displayed (symbol-mode directories).
func iconFor(mode iconMode, entry fileEntry) *iconInfo {
	if mode == iconNerd {
		s := devicons.IconForPath(entry.path)
		return &iconInfo{char: s.Icon, color: s.Color}
	}
	if entry.isDir {
		return nil
	}
	if entry.isBinary {
		return &symbolBinary
	}
	return &symbolText
}

// prefix returns the icon character followed by a space,
// or empty string if nil.
func (i *iconInfo) prefix() string {
	if i == nil {
		return ""
	}
	return i.char + " "
}

const ansiFgReset = termenv.CSI + "39m"

// colorize finds the icon character in line and injects
// ANSI foreground color around it. Returns line unchanged if nil.
func (i *iconInfo) colorize(line string) string {
	if i == nil {
		return line
	}
	pos := strings.Index(line, i.char)
	if pos < 0 {
		return line
	}
	set := termenv.CSI +
		termenv.RGBColor(i.color).Sequence(false) + "m"
	iconEnd := pos + len(i.char)
	return line[:pos] + set + line[pos:iconEnd] + ansiFgReset + line[iconEnd:]
}
