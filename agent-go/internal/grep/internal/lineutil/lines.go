package lineutil

import "bytes"

// Line represents a single line with its position metadata.
type Line struct {
	Number int    // 1-based line number
	Start  int    // byte offset of line start in the original data
	End    int    // byte offset of line end (exclusive, before '\n')
	Data   []byte // line content (without trailing '\n' or '\r\n')
}

// FindLineStart returns the byte offset of the start of the line
// containing the byte at position pos.
func FindLineStart(data []byte, pos int) int {
	if pos <= 0 {
		return 0
	}
	idx := bytes.LastIndexByte(data[:pos], '\n')
	if idx < 0 {
		return 0
	}
	return idx + 1
}

// FindLineEnd returns the byte offset of the end of the line
// containing the byte at position pos (offset of the '\n' or end of data).
func FindLineEnd(data []byte, pos int) int {
	idx := bytes.IndexByte(data[pos:], '\n')
	if idx < 0 {
		return len(data)
	}
	return pos + idx
}

// SplitLines returns all lines in data with their byte offsets.
func SplitLines(data []byte) []Line {
	if len(data) == 0 {
		return nil
	}
	// Count newlines to pre-allocate
	n := bytes.Count(data, []byte{'\n'}) + 1
	lines := make([]Line, 0, n)
	lineNum := 1
	start := 0
	for i := range data {
		if data[i] == '\n' {
			end := i
			// Strip trailing \r for Windows line endings
			if end > start && data[end-1] == '\r' {
				end--
			}
			lines = append(lines, Line{
				Number: lineNum,
				Start:  start,
				End:    end,
				Data:   data[start:end],
			})
			lineNum++
			start = i + 1
		}
	}
	// Handle last line without trailing newline
	if start <= len(data) {
		end := len(data)
		if end > start && data[end-1] == '\r' {
			end--
		}
		lines = append(lines, Line{
			Number: lineNum,
			Start:  start,
			End:    end,
			Data:   data[start:end],
		})
	}
	return lines
}
