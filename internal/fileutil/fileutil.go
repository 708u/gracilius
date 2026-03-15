package fileutil

import (
	"bufio"
	"bytes"
)

// IsBinary returns true if data appears to be binary.
// It checks the first 8192 bytes for null bytes.
func IsBinary(data []byte) bool {
	checkSize := min(len(data), 8192)
	for i := range checkSize {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// SplitLines splits data into lines.
// Uses bufio.Scanner which splits on \n and strips trailing \r from each line.
func SplitLines(data []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
