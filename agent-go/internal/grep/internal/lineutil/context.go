package lineutil

// ContextTracker manages before/after context lines around matches,
// ensuring overlapping contexts don't produce duplicate lines.
type ContextTracker struct {
	before int
	after  int
	lines  []Line

	lastEmittedLine int // last line index included in output
}

// NewContextTracker creates a tracker for the given context window and line set.
func NewContextTracker(before, after int, lines []Line) *ContextTracker {
	return &ContextTracker{
		before:          before,
		after:           after,
		lines:           lines,
		lastEmittedLine: -1,
	}
}

// GetContext returns the before/after context lines for a match at lineIdx.
func (ct *ContextTracker) GetContext(lineIdx int) (before, after []string) {
	// Before context
	startBefore := max(lineIdx-ct.before, 0)
	// Don't repeat lines already emitted by a previous match's after-context
	if startBefore <= ct.lastEmittedLine {
		startBefore = ct.lastEmittedLine + 1
	}
	for i := startBefore; i < lineIdx; i++ {
		before = append(before, string(ct.lines[i].Data))
	}

	// After context
	endAfter := min(lineIdx+ct.after+1, len(ct.lines))
	for i := lineIdx + 1; i < endAfter; i++ {
		after = append(after, string(ct.lines[i].Data))
	}

	ct.lastEmittedLine = max(endAfter-1, lineIdx)

	return before, after
}
