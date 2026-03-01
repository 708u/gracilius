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
	if entry.isBinary {
		return &symbolBinary
	}
	return &symbolText
}

const ansiFgReset = termenv.CSI + "39m"

// colorize injects ANSI foreground color around the icon position
// in a line. pos is the byte offset of the icon character.
// This preserves any existing ANSI (e.g. background) on the line.
func (i *iconInfo) colorize(line string, pos int) string {
	set := termenv.CSI +
		termenv.RGBColor(i.color).Sequence(false) + "m"
	iconEnd := pos + len(i.char)
	return line[:pos] + set + line[pos:iconEnd] + ansiFgReset + line[iconEnd:]
}
