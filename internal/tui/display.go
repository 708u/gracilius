package tui

import "strings"

// displayWidth computes the display width of a string (simplified).
func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if r >= 0x1100 && isWideRune(r) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

// isWideRune returns true if the rune is a wide (CJK) character.
func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) ||
		(r >= 0x2E80 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE1F) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFF)
}

// truncateString truncates a string to the given display width.
func truncateString(s string, width int) string {
	if displayWidth(s) <= width {
		return s
	}
	if width <= 3 {
		result := ""
		w := 0
		for _, r := range s {
			rw := 1
			if isWideRune(r) {
				rw = 2
			}
			if w+rw > width {
				break
			}
			result += string(r)
			w += rw
		}
		return result
	}
	result := ""
	w := 0
	targetWidth := width - 3
	for _, r := range s {
		rw := 1
		if isWideRune(r) {
			rw = 2
		}
		if w+rw > targetWidth {
			break
		}
		result += string(r)
		w += rw
	}
	return result + "..."
}

// padRight pads a string with spaces to the given display width.
func padRight(s string, width int) string {
	currentWidth := displayWidth(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// expandTabs replaces tabs with 4 spaces.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}
