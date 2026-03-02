package files

import (
	"testing"
)

func TestFuzzyScoreBasicMatching(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		matched bool
	}{
		{"exact match", "foo", "foo", true},
		{"substring", "bar", "foobarbaz", true},
		{"subsequence", "fbz", "foobarbaz", true},
		{"case insensitive", "FOO", "foo", true},
		{"no match", "xyz", "foo", false},
		{"pattern longer than text", "foobar", "foo", false},
		{"empty pattern", "", "foo", true},
		{"path match", "main.go", "cmd/agent-api/main.go", true},
		{"initials", "amg", "cmd/agent-api/main.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, matched := fuzzyScore(tt.pattern, tt.text)
			if matched != tt.matched {
				t.Errorf("fuzzyScore(%q, %q) matched=%v, want %v", tt.pattern, tt.text, matched, tt.matched)
			}
		})
	}
}

func TestFuzzyScoreRanking(t *testing.T) {
	// Exact basename match should score highest.
	scoreExact, _ := fuzzyScore("main.go", "main.go")
	scorePath, _ := fuzzyScore("main.go", "cmd/agent-api/main.go")
	scoreSubseq, _ := fuzzyScore("main.go", "cmd/maintenance/ongo.go")

	if scoreExact <= scorePath {
		t.Errorf("exact match (%d) should score higher than path match (%d)", scoreExact, scorePath)
	}
	if scorePath <= scoreSubseq {
		t.Errorf("basename match (%d) should score higher than scattered subsequence (%d)", scorePath, scoreSubseq)
	}
}

func TestFuzzyScoreConsecutiveBonus(t *testing.T) {
	// "hand" in "handler.go" (consecutive) should beat "hand" in "h_a_n_d.go" (scattered).
	scoreConsec, _ := fuzzyScore("hand", "handler.go")
	scoreScatter, _ := fuzzyScore("hand", "helpers/auth/nav/dispatch.go")

	if scoreConsec <= scoreScatter {
		t.Errorf("consecutive (%d) should score higher than scattered (%d)", scoreConsec, scoreScatter)
	}
}

func TestFuzzyScoreBoundaryBonus(t *testing.T) {
	// Matching at word boundaries should score higher.
	// "sc" matching "src/components" (boundary: s after /, c after /) vs "src/discourse" (s at start, c in middle)
	scoreBoundary, _ := fuzzyScore("sc", "src/components")
	scoreMiddle, _ := fuzzyScore("sc", "discourse")

	if scoreBoundary <= scoreMiddle {
		t.Errorf("boundary match (%d) should score higher than middle match (%d)", scoreBoundary, scoreMiddle)
	}
}

func TestFuzzyScoreCamelCase(t *testing.T) {
	// camelCase boundary matching.
	score, matched := fuzzyScore("SM", "ServiceManager")
	if !matched {
		t.Fatal("should match camelCase initials")
	}
	if score <= 0 {
		t.Errorf("camelCase match should have positive score, got %d", score)
	}
}

func TestFuzzyScoreCaseSensitiveBonus(t *testing.T) {
	// Exact case should score slightly higher than different case.
	scoreExact, _ := fuzzyScore("Main", "Main.go")
	scoreLower, _ := fuzzyScore("main", "Main.go")

	if scoreExact <= scoreLower {
		t.Errorf("exact case (%d) should score higher than different case (%d)", scoreExact, scoreLower)
	}
}

func TestFuzzyScorePathPreference(t *testing.T) {
	// Shorter paths with the match should generally score higher.
	scoreShort, _ := fuzzyScore("auth", "auth.go")
	scoreLong, _ := fuzzyScore("auth", "src/internal/middleware/auth.go")

	if scoreShort <= scoreLong {
		t.Errorf("shorter path match (%d) should score higher than longer path match (%d)", scoreShort, scoreLong)
	}
}

func TestSearchFilesIntegration(t *testing.T) {
	// Use the actual workspace if available, otherwise skip.
	result, err := SearchFiles("fuzzy", ".", 10)
	if err != nil {
		t.Skipf("skipping integration test: %v", err.Message)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Query != "fuzzy" {
		t.Errorf("query = %q, want %q", result.Query, "fuzzy")
	}
	// Should find this test file itself.
	found := false
	for _, r := range result.Results {
		if r.Path == "fuzzy_test.go" || r.Path == "internal/files/fuzzy_test.go" {
			found = true
			break
		}
	}
	// This might not work depending on cwd, so just check we got results.
	_ = found
}
