package gogrep

// Match represents a single match within a file.
type Match struct {
	Path       string   // file path (relative to search root)
	LineNumber int      // 1-based line number of the match
	Column     int      // 0-based byte offset within the line
	Line       string   // the matched line content (without trailing newline)
	Before     []string // context lines before the match
	After      []string // context lines after the match
}

// FileMatches groups all matches within a single file.
type FileMatches struct {
	Path    string
	Matches []Match
	Count   int // total match count in this file
}

// Results holds the complete output of a grep operation.
type Results struct {
	Files      []FileMatches // one entry per file with matches
	TotalCount int           // total matches across all files
	Truncated  bool          // true if HeadLimit caused early termination
}
