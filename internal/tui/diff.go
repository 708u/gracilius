package tui

// diffLine represents a single line of a diff.
type diffLine struct {
	op   rune // ' ' = unchanged, '+' = added, '-' = removed
	text string
}

// computeLineDiff computes the diff between two slices of lines using LCS.
func computeLineDiff(oldLines, newLines []string) []diffLine {
	m, n := len(oldLines), len(newLines)

	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else {
				lcs[i][j] = max(lcs[i-1][j], lcs[i][j-1])
			}
		}
	}

	var result []diffLine
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			result = append(result, diffLine{' ', oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			result = append(result, diffLine{'+', newLines[j-1]})
			j--
		} else {
			result = append(result, diffLine{'-', oldLines[i-1]})
			i--
		}
	}

	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}

	return result
}
