package literal

import (
	"regexp/syntax"
	"unicode/utf8"
)

const (
	maxLiterals   = 64  // max number of literal alternatives to track
	maxLiteralLen = 128 // max byte length of a single literal
)

// Extract analyzes a parsed regex AST and extracts literal sequences
// that must appear in any match. These can be used as fast prefilters.
func Extract(re *syntax.Regexp) *Literals {
	info := analyze(re)
	return info.toLiterals()
}

type extractInfo struct {
	prefixes [][]byte // literal byte sequences all matches must start with
	exact    bool     // all prefixes are exact (no further regex needed)
	cut      bool     // extraction was cut short (too many/long literals)
}

func analyze(re *syntax.Regexp) *extractInfo {
	switch re.Op {
	case syntax.OpLiteral:
		return analyzeLiteral(re)
	case syntax.OpConcat:
		return analyzeConcat(re)
	case syntax.OpAlternate:
		return analyzeAlternate(re)
	case syntax.OpCapture:
		return analyze(re.Sub[0])
	case syntax.OpPlus:
		info := analyze(re.Sub[0])
		info.exact = false // x+ is never exact
		return info
	case syntax.OpRepeat:
		if re.Min >= 1 {
			info := analyze(re.Sub[0])
			info.exact = false
			return info
		}
		return &extractInfo{} // x{0,...} might not occur
	case syntax.OpQuest, syntax.OpStar:
		return &extractInfo{} // optional elements yield no required literals
	case syntax.OpEmptyMatch:
		return &extractInfo{exact: true}
	case syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary,
		syntax.OpNoWordBoundary:
		// Zero-width assertions constrain matching beyond the literal,
		// so the literal alone is not sufficient (exact = false).
		return &extractInfo{exact: false}
	default:
		// OpCharClass, OpAnyChar, OpAnyCharNotNL, OpNoMatch
		return &extractInfo{}
	}
}

func analyzeLiteral(re *syntax.Regexp) *extractInfo {
	var buf []byte
	for _, r := range re.Rune {
		var tmp [utf8.UTFMax]byte
		n := utf8.EncodeRune(tmp[:], r)
		buf = append(buf, tmp[:n]...)
	}
	if len(buf) == 0 {
		return &extractInfo{exact: true}
	}
	if len(buf) > maxLiteralLen {
		buf = buf[:maxLiteralLen]
		return &extractInfo{prefixes: [][]byte{buf}, exact: false, cut: true}
	}
	isCaseInsensitive := re.Flags&syntax.FoldCase != 0
	if isCaseInsensitive {
		// For case-insensitive, use the literal as-is but mark inexact
		// The prefilter will handle case folding
		return &extractInfo{prefixes: [][]byte{toLowerBytes(buf)}, exact: false}
	}
	return &extractInfo{prefixes: [][]byte{buf}, exact: true}
}

func analyzeConcat(re *syntax.Regexp) *extractInfo {
	result := &extractInfo{exact: true}
	for _, sub := range re.Sub {
		subInfo := analyze(sub)
		result = concatInfos(result, subInfo)
		if result.cut || !result.exact {
			break
		}
	}
	return result
}

func analyzeAlternate(re *syntax.Regexp) *extractInfo {
	var allPrefixes [][]byte
	allExact := true
	for _, sub := range re.Sub {
		subInfo := analyze(sub)
		if len(subInfo.prefixes) == 0 {
			return &extractInfo{}
		}
		allPrefixes = append(allPrefixes, subInfo.prefixes...)
		if !subInfo.exact {
			allExact = false
		}
		if len(allPrefixes) > maxLiterals {
			return &extractInfo{
				prefixes: allPrefixes[:maxLiterals],
				exact:    false,
				cut:      true,
			}
		}
	}
	return &extractInfo{prefixes: allPrefixes, exact: allExact}
}

func concatInfos(left, right *extractInfo) *extractInfo {
	if len(left.prefixes) == 0 {
		return right
	}
	if len(right.prefixes) == 0 {
		left.exact = false
		return left
	}
	var combined [][]byte
	for _, lp := range left.prefixes {
		for _, rp := range right.prefixes {
			c := make([]byte, len(lp)+len(rp))
			copy(c, lp)
			copy(c[len(lp):], rp)
			if len(c) > maxLiteralLen {
				c = c[:maxLiteralLen]
			}
			combined = append(combined, c)
			if len(combined) > maxLiterals {
				return &extractInfo{
					prefixes: combined[:maxLiterals],
					exact:    false,
					cut:      true,
				}
			}
		}
	}
	return &extractInfo{
		prefixes: combined,
		exact:    left.exact && right.exact,
	}
}

func (info *extractInfo) toLiterals() *Literals {
	if len(info.prefixes) == 0 {
		return &Literals{Position: "none"}
	}
	seqs := make([]Seq, len(info.prefixes))
	for i, p := range info.prefixes {
		seqs[i] = Seq{Bytes: p, Exact: info.exact}
	}
	return &Literals{
		Seqs:     seqs,
		Position: "prefix",
	}
}

func toLowerBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + 32
		} else {
			out[i] = c
		}
	}
	return out
}
