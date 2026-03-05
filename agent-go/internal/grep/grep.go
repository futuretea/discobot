package gogrep

import (
	"bytes"
	"context"
	"fmt"
	"regexp/syntax"

	regexp "github.com/grafana/regexp"

	"github.com/obot-platform/discobot/agent-go/internal/grep/literal"
)

// GrepOptions configures a grep operation.
type GrepOptions struct {
	Pattern          string
	Path             string
	Type             string // file type filter: "go", "js", "py", etc.
	Glob             string // glob pattern filter: "*.tsx", "src/**/*.go"
	OutputMode       string // "content" (default), "files_with_matches", "count"
	CaseInsensitive  bool
	Context          int // lines of context around match (-C)
	After            int // lines after match (-A)
	Before           int // lines before match (-B)
	LineNumbers      bool
	HeadLimit        int // limit output entries
	Offset           int // skip first N entries
	Multiline        bool
	RespectGitignore *bool // nil = auto (true if in git repo), true/false = explicit
}

// Grep performs a grep operation and returns structured results.
func Grep(ctx context.Context, opts GrepOptions) (*Results, error) {
	opts = normalizeOptions(opts)

	// Parse regex via regexp/syntax for literal extraction
	flags := syntax.Perl
	if opts.CaseInsensitive {
		flags |= syntax.FoldCase
	}
	if opts.Multiline {
		flags |= syntax.DotNL
	}
	syntaxTree, err := syntax.Parse(opts.Pattern, flags)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	syntaxTree = syntaxTree.Simplify()

	// Extract literals from the regex AST
	lits := literal.Extract(syntaxTree)

	// Compile the full regex for verification
	re, err := regexp.Compile(buildRegexString(opts))
	if err != nil {
		return nil, fmt.Errorf("compile pattern: %w", err)
	}

	// Check if all extracted literals are exact (no regex verification needed)
	exactMatch := false
	foldCase := false
	var exactLiterals [][]byte
	if !lits.IsEmpty() && !opts.Multiline {
		allExact := true
		for _, seq := range lits.Seqs {
			if !seq.Exact {
				allExact = false
				break
			}
		}
		if allExact {
			exactMatch = true
			exactLiterals = make([][]byte, len(lits.Seqs))
			for i, seq := range lits.Seqs {
				if opts.CaseInsensitive {
					exactLiterals[i] = bytes.ToLower(seq.Bytes)
				} else {
					exactLiterals[i] = seq.Bytes
				}
			}
			foldCase = opts.CaseInsensitive
		}
	}

	// For case-insensitive patterns: if the pattern is a pure literal (no regex
	// metacharacters), use the exact match fast path with case folding even though
	// the literal extractor marks FoldCase patterns as inexact.
	if !exactMatch && opts.CaseInsensitive && !opts.Multiline && isPlainLiteral(opts.Pattern) {
		exactMatch = true
		foldCase = true
		exactLiterals = [][]byte{bytes.ToLower([]byte(opts.Pattern))}
	}

	s := &searcher{
		re:         re,
		opts:       opts,
		exactMatch: exactMatch,
		foldCase:   foldCase,
		literals:   exactLiterals,
	}

	return walk(ctx, opts, s)
}

func normalizeOptions(opts GrepOptions) GrepOptions {
	if opts.OutputMode == "" {
		opts.OutputMode = "content"
	}
	if opts.OutputMode == "content" {
		opts.LineNumbers = true
	}
	if opts.Context > 0 {
		if opts.Before == 0 {
			opts.Before = opts.Context
		}
		if opts.After == 0 {
			opts.After = opts.Context
		}
	}
	return opts
}

// isPlainLiteral checks if a pattern contains no regex metacharacters.
func isPlainLiteral(pattern string) bool {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\', '.', '+', '*', '?', '(', ')', '[', ']', '{', '}', '|', '^', '$':
			return false
		}
	}
	return true
}

func buildRegexString(opts GrepOptions) string {
	prefix := ""
	if opts.CaseInsensitive {
		prefix += "(?i)"
	}
	if opts.Multiline {
		prefix += "(?s)"
	}
	return prefix + opts.Pattern
}
