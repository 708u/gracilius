package render

import (
	"github.com/muesli/termenv"
)

// Theme holds the color configuration for the TUI.
type Theme struct {
	Name                string // Chroma style name
	SelectionBg         string // Editor selection background hex color
	ListSelectionBg     string // List/tree active selection hex color
	ActiveFileBg        string // File tree active-tab file background hex color
	TabActiveFg         string // Active tab foreground hex color
	TabActiveBorder     string // Active tab underline hex color
	TabInactiveFg       string // Inactive tab foreground hex color
	OpenFileSelectionBg string // Open-file overlay selection bg
	OpenFileMatchFg     string // Fuzzy match highlight fg
	LogoLeaf            string // Welcome logo top color (green/leaf)
	LogoTrunk           string // Welcome logo bottom color (brown/trunk)
	SearchMatchBg       string // Search match background hex color
	SearchCurrentBg     string // Current search match background hex color
}

// SelectionBgSeq returns the ANSI SGR sequence for editor selection background.
func (t Theme) SelectionBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.SelectionBg).Sequence(true) + "m"
}

// SearchMatchBgSeq returns the ANSI SGR sequence for search match background.
func (t Theme) SearchMatchBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.SearchMatchBg).Sequence(true) + "m"
}

// SearchCurrentBgSeq returns the ANSI SGR sequence for current search match background.
func (t Theme) SearchCurrentBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.SearchCurrentBg).Sequence(true) + "m"
}

// ListSelectionBgSeq returns the ANSI SGR sequence for list selection background.
func (t Theme) ListSelectionBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.ListSelectionBg).Sequence(true) + "m"
}

// ActiveFileBgSeq returns the ANSI SGR sequence for active file background.
func (t Theme) ActiveFileBgSeq() string {
	return termenv.CSI + termenv.RGBColor(t.ActiveFileBg).Sequence(true) + "m"
}

// Dark is the dark theme configuration.
var Dark = Theme{
	Name:                "github-dark",
	SelectionBg:         "#264F78",
	ListSelectionBg:     "#505050",
	ActiveFileBg:        "#2A2D2E",
	TabActiveFg:         "#FFFFFF",
	TabActiveBorder:     "#E8AB53",
	TabInactiveFg:       "#969696",
	OpenFileSelectionBg: "#04395E",
	OpenFileMatchFg:     "#FFCC66",
	LogoLeaf:            "#73C991",
	LogoTrunk:           "#CE9178",
	SearchMatchBg:       "#613214",
	SearchCurrentBg:     "#9E6A03",
}

// Light is the light theme configuration.
var Light = Theme{
	Name:                "github",
	SelectionBg:         "#ADD6FF",
	ListSelectionBg:     "#B8D8F8",
	ActiveFileBg:        "#E4E6F1",
	TabActiveFg:         "#333333",
	TabActiveBorder:     "#005FB8",
	TabInactiveFg:       "#6E6E6E",
	OpenFileSelectionBg: "#C4E0F9",
	OpenFileMatchFg:     "#0066CC",
	LogoLeaf:            "#1B7F37",
	LogoTrunk:           "#795E26",
	SearchMatchBg:       "#FFF2CC",
	SearchCurrentBg:     "#FFD700",
}
