package gogrep

import (
	"bytes"
	"io"
	"os"
	"sort"
	"sync"

	regexp "github.com/grafana/regexp"

	"github.com/obot-platform/discobot/agent-go/internal/grep/internal/lineutil"
)

const maxPoolBufSize = 1 << 20 // 1MB - don't pool buffers larger than this

var bufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, maxPoolBufSize)
		return &buf
	},
}

type searcher struct {
	re         *regexp.Regexp
	opts       GrepOptions
	exactMatch bool     // true when prefilter alone is sufficient (no regex needed)
	foldCase   bool     // true when case-insensitive exact match (fold data to lowercase)
	literals   [][]byte // exact literal patterns for regex-free matching
}

// readFile reads a file into a pooled buffer. sizeHint avoids an fstat syscall
// when the caller already knows the file size (e.g. from DirEntry.Info()).
// The caller must call the returned cleanup function when done with the data.
func readFile(path string, sizeHint int64) (data []byte, cleanup func(), err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	size := int(sizeHint)
	if size <= 0 {
		fi, err := f.Stat()
		if err != nil {
			return nil, nil, err
		}
		size = int(fi.Size())
	}
	if size == 0 {
		return nil, func() {}, nil
	}

	bp := bufPool.Get().(*[]byte)
	buf := *bp
	if cap(buf) < size {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}

	_, err = io.ReadFull(f, buf)
	if err != nil {
		*bp = buf[:0]
		bufPool.Put(bp)
		return nil, nil, err
	}

	return buf, func() {
		if cap(buf) <= maxPoolBufSize {
			*bp = buf[:0]
			bufPool.Put(bp)
		}
	}, nil
}

// searchFile searches a single file and returns matches.
func (s *searcher) searchFile(path string, sizeHint int64) (*FileMatches, error) {
	data, cleanup, err := readFile(path, sizeHint)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if len(data) == 0 {
		return nil, nil
	}

	// Skip binary files (check for NUL in first 512 bytes)
	checkLen := min(len(data), 512)
	if bytes.IndexByte(data[:checkLen], 0) >= 0 {
		return nil, nil
	}

	// Fast path for count and files_with_matches: avoid SplitLines allocation
	if !s.opts.Multiline && (s.opts.OutputMode == "count" || s.opts.OutputMode == "files_with_matches") {
		return s.searchFast(path, data)
	}

	if s.opts.Multiline {
		return s.searchMultiline(path, data)
	}
	return s.searchLines(path, data)
}

// searchFast scans the buffer line-by-line without allocating a Line slice.
// Used for count and files_with_matches modes where we don't need context or line content.
func (s *searcher) searchFast(path string, data []byte) (*FileMatches, error) {
	count := 0
	pos := 0
	for pos < len(data) {
		// Find end of current line
		nl := bytes.IndexByte(data[pos:], '\n')
		var line []byte
		if nl < 0 {
			line = data[pos:]
			pos = len(data)
		} else {
			line = data[pos : pos+nl]
			pos = pos + nl + 1
		}

		// Strip \r
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		// Match: exact literal path or regex
		matched := false
		if s.exactMatch {
			if s.foldCase {
				matched = containsAnyLiteralFold(line, s.literals)
			} else {
				matched = containsAnyLiteral(line, s.literals)
			}
		} else {
			matched = s.re.Match(line)
		}
		if matched {
			count++
			if s.opts.OutputMode == "files_with_matches" {
				return &FileMatches{Path: path, Count: 1}, nil
			}
		}
	}

	if count == 0 {
		return nil, nil
	}
	return &FileMatches{Path: path, Count: count}, nil
}

func (s *searcher) searchLines(path string, data []byte) (*FileMatches, error) {
	needContext := s.opts.Before > 0 || s.opts.After > 0

	// If we need context lines, use the SplitLines path (simpler, correctness matters)
	if needContext {
		return s.searchLinesWithContext(path, data)
	}

	// Fast inline scan: no SplitLines allocation needed
	var matches []Match
	lineNum := 1
	pos := 0
	for pos < len(data) {
		nl := bytes.IndexByte(data[pos:], '\n')
		var line []byte
		var lineEnd int
		if nl < 0 {
			line = data[pos:]
			lineEnd = len(data)
		} else {
			line = data[pos : pos+nl]
			lineEnd = pos + nl + 1
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		var loc []int
		if s.exactMatch {
			if s.foldCase {
				loc = findAnyLiteralFold(line, s.literals)
			} else {
				loc = findAnyLiteral(line, s.literals)
			}
		} else {
			loc = s.re.FindIndex(line)
		}
		if loc != nil {
			if s.opts.OutputMode == "files_with_matches" {
				return &FileMatches{Path: path, Count: 1}, nil
			}
			matches = append(matches, Match{
				Path:       path,
				LineNumber: lineNum,
				Column:     loc[0],
				Line:       string(line),
			})
		}

		lineNum++
		pos = lineEnd
	}

	if len(matches) == 0 {
		return nil, nil
	}
	return &FileMatches{Path: path, Matches: matches, Count: len(matches)}, nil
}

// searchLinesWithContext handles the content mode with before/after context lines.
// Uses SplitLines since context tracking needs random access to surrounding lines.
func (s *searcher) searchLinesWithContext(path string, data []byte) (*FileMatches, error) {
	lines := lineutil.SplitLines(data)
	var matches []Match
	ct := lineutil.NewContextTracker(s.opts.Before, s.opts.After, lines)

	for i, line := range lines {
		var loc []int
		if s.exactMatch {
			if s.foldCase {
				loc = findAnyLiteralFold(line.Data, s.literals)
			} else {
				loc = findAnyLiteral(line.Data, s.literals)
			}
		} else {
			loc = s.re.FindIndex(line.Data)
		}
		if loc == nil {
			continue
		}

		if s.opts.OutputMode == "files_with_matches" {
			return &FileMatches{Path: path, Count: 1}, nil
		}

		m := Match{
			Path:       path,
			LineNumber: line.Number,
			Column:     loc[0],
			Line:       string(line.Data),
		}
		m.Before, m.After = ct.GetContext(i)
		matches = append(matches, m)
	}

	if len(matches) == 0 {
		return nil, nil
	}
	return &FileMatches{Path: path, Matches: matches, Count: len(matches)}, nil
}

func (s *searcher) searchMultiline(path string, data []byte) (*FileMatches, error) {
	allLocs := s.re.FindAllIndex(data, -1)
	if len(allLocs) == 0 {
		return nil, nil
	}

	if s.opts.OutputMode == "files_with_matches" {
		return &FileMatches{Path: path, Count: len(allLocs)}, nil
	}

	lines := lineutil.SplitLines(data)
	var matches []Match

	for _, loc := range allLocs {
		lineIdx := findLineIndex(lines, loc[0])
		if lineIdx < 0 {
			continue
		}

		m := Match{
			Path:       path,
			LineNumber: lines[lineIdx].Number,
			Column:     loc[0] - lines[lineIdx].Start,
			Line:       string(data[loc[0]:loc[1]]),
		}
		matches = append(matches, m)
	}

	if len(matches) == 0 {
		return nil, nil
	}

	return &FileMatches{
		Path:    path,
		Matches: matches,
		Count:   len(matches),
	}, nil
}

// containsAnyLiteral checks if data contains any of the exact literal patterns.
func containsAnyLiteral(data []byte, literals [][]byte) bool {
	for _, lit := range literals {
		if bytes.Contains(data, lit) {
			return true
		}
	}
	return false
}

// findAnyLiteral returns [start, end] of the first literal found in data, or nil.
func findAnyLiteral(data []byte, literals [][]byte) []int {
	bestIdx := -1
	bestEnd := 0
	for _, lit := range literals {
		idx := bytes.Index(data, lit)
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			bestEnd = idx + len(lit)
		}
	}
	if bestIdx < 0 {
		return nil
	}
	return []int{bestIdx, bestEnd}
}

// containsAnyLiteralFold is like containsAnyLiteral but case-insensitive.
// The literals must already be lowercased.
func containsAnyLiteralFold(data []byte, literals [][]byte) bool {
	for _, lit := range literals {
		if indexFold(data, lit) >= 0 {
			return true
		}
	}
	return false
}

// findAnyLiteralFold is like findAnyLiteral but case-insensitive.
// The literals must already be lowercased.
func findAnyLiteralFold(data []byte, literals [][]byte) []int {
	bestIdx := -1
	bestEnd := 0
	for _, lit := range literals {
		idx := indexFold(data, lit)
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			bestEnd = idx + len(lit)
		}
	}
	if bestIdx < 0 {
		return nil
	}
	return []int{bestIdx, bestEnd}
}

// indexFold returns the index of the first case-insensitive match of needle in data.
// needle must be lowercase. Uses bytes.EqualFold for correctness with Unicode.
func indexFold(data, needle []byte) int {
	if len(needle) == 0 {
		return 0
	}
	if len(needle) > len(data) {
		return -1
	}
	// Fast path: ASCII-only needle — use manual lowering to avoid allocation
	asciiNeedle := true
	for _, b := range needle {
		if b >= 0x80 {
			asciiNeedle = false
			break
		}
	}
	if asciiNeedle {
		return indexFoldASCII(data, needle)
	}
	// Fallback: use bytes.EqualFold for full Unicode support
	end := len(data) - len(needle) + 1
	for i := range end {
		if bytes.EqualFold(data[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

// indexFoldASCII is a fast case-insensitive search for ASCII-only needles.
// needle must be lowercase ASCII.
func indexFoldASCII(data, needle []byte) int {
	n := len(needle)
	first := needle[0]
	end := len(data) - n + 1
	for i := range end {
		b := data[i]
		// Quick check on first byte
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if b != first {
			continue
		}
		// Check remaining bytes
		match := true
		for j := 1; j < n; j++ {
			c := data[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func findLineIndex(lines []lineutil.Line, byteOffset int) int {
	idx := sort.Search(len(lines), func(i int) bool {
		return lines[i].End >= byteOffset
	})
	if idx < len(lines) && lines[idx].Start <= byteOffset {
		return idx
	}
	return -1
}
