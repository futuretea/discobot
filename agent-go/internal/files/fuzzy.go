package files

import (
	"strings"
	"unicode"
)

// Scoring constants modeled after fzf's scoring algorithm.
const (
	scoreMatch        = 16
	scoreGapStart     = -3
	scoreGapExtension = -1

	// Bonuses
	bonusConsecutive    = 8
	bonusBoundary       = scoreMatch / 2 // word boundary (after /, ., _, -, space)
	bonusCamelCase      = bonusBoundary - 1
	bonusFirstCharMatch = bonusBoundary + 2
	bonusExactBasename  = 32
)

// fuzzyScore computes a fuzzy match score for pattern against text.
// Returns (score, matched). Score is 0 if matched is false.
// The algorithm matches pattern characters as a subsequence of text,
// rewarding consecutive runs, word-boundary matches, and penalizing gaps.
func fuzzyScore(pattern, text string) (int, bool) {
	pLen := len(pattern)
	tLen := len(text)

	if pLen == 0 {
		return 0, true
	}
	if pLen > tLen {
		return 0, false
	}

	// Quick check: does text contain all pattern chars as a subsequence?
	pi := 0
	for ti := 0; ti < tLen && pi < pLen; ti++ {
		if toLowerByte(text[ti]) == toLowerByte(pattern[pi]) {
			pi++
		}
	}
	if pi < pLen {
		return 0, false
	}

	// Find optimal alignment using a greedy forward pass with scoring.
	// For each pattern char, find the best position considering bonuses.
	score, positions := bestAlignment(pattern, text)

	// Extra bonus: if the basename of the path matches the full pattern exactly.
	lastSlash := strings.LastIndexByte(text, '/')
	basename := text
	if lastSlash >= 0 {
		basename = text[lastSlash+1:]
	}
	if strings.EqualFold(basename, pattern) {
		score += bonusExactBasename
	}

	_ = positions // positions could be used for highlighting in the future
	return score, true
}

// bestAlignment finds the highest-scoring alignment of pattern in text.
// Uses a two-pass approach: forward pass to find the rightmost match positions,
// then backward pass from each endpoint to find the tightest (highest-scoring) match.
func bestAlignment(pattern, text string) (int, []int) {
	pLen := len(pattern)
	tLen := len(text)

	// Forward pass: find the first valid match (leftmost positions).
	positions := make([]int, pLen)
	pi := 0
	for ti := 0; ti < tLen && pi < pLen; ti++ {
		if toLowerByte(text[ti]) == toLowerByte(pattern[pi]) {
			positions[pi] = ti
			pi++
		}
	}

	// Score the found alignment.
	score := scoreAlignment(pattern, text, positions)

	// Try an alternative alignment: for each pattern char, prefer boundary positions.
	altPositions := boundaryPreferredAlignment(pattern, text)
	if altPositions != nil {
		altScore := scoreAlignment(pattern, text, altPositions)
		if altScore > score {
			score = altScore
			positions = altPositions
		}
	}

	return score, positions
}

// boundaryPreferredAlignment tries to align pattern chars at word boundaries.
func boundaryPreferredAlignment(pattern, text string) []int {
	pLen := len(pattern)
	tLen := len(text)
	positions := make([]int, pLen)
	pi := 0

	for ti := 0; ti < tLen && pi < pLen; ti++ {
		if toLowerByte(text[ti]) != toLowerByte(pattern[pi]) {
			continue
		}

		// Prefer this position if it's a boundary or we have no better option.
		if pi == 0 || isBoundary(text, ti) || ti == positions[pi-1]+1 {
			positions[pi] = ti
			pi++
		} else {
			// Still take it if we haven't found a spot yet — greedy fallback.
			positions[pi] = ti
			pi++
		}
	}

	if pi < pLen {
		return nil
	}
	return positions
}

// scoreAlignment scores a specific alignment of pattern positions in text.
func scoreAlignment(pattern, text string, positions []int) int {
	pLen := len(positions)
	if pLen == 0 {
		return 0
	}

	score := 0

	for i, pos := range positions {
		// Base match score.
		score += scoreMatch

		// Case-sensitive exact match bonus.
		if text[pos] == pattern[i] {
			score++
		}

		// First character bonus.
		if pos == 0 {
			score += bonusFirstCharMatch
		}

		// Boundary bonus.
		if isBoundary(text, pos) {
			score += bonusBoundary
		} else if isCamelBoundary(text, pos) {
			score += bonusCamelCase
		}

		// Consecutive bonus or gap penalty.
		if i > 0 {
			gap := pos - positions[i-1] - 1
			if gap == 0 {
				score += bonusConsecutive
			} else {
				score += scoreGapStart + scoreGapExtension*(gap-1)
			}
		}
	}

	// Prefer shorter total span.
	span := positions[pLen-1] - positions[0] + 1
	score -= (span - pLen) // penalize unused chars in the span

	return score
}

// isBoundary returns true if position pos in text is at a word boundary.
// A boundary is the first char, or follows a separator (/, ., _, -, space).
func isBoundary(text string, pos int) bool {
	if pos == 0 {
		return true
	}
	prev := text[pos-1]
	return prev == '/' || prev == '.' || prev == '_' || prev == '-' || prev == ' '
}

// isCamelBoundary returns true if position pos is a camelCase boundary
// (lowercase followed by uppercase).
func isCamelBoundary(text string, pos int) bool {
	if pos == 0 {
		return false
	}
	return unicode.IsLower(rune(text[pos-1])) && unicode.IsUpper(rune(text[pos]))
}

// toLowerByte converts an ASCII byte to lowercase.
func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}
