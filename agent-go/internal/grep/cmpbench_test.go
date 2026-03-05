package gogrep

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const defaultCorpus = "/usr/local/go/src"
const rgBinary = "/usr/bin/rg"

func boolPtr(b bool) *bool { return &b }

func benchCorpus() string {
	if c := os.Getenv("GOGREP_BENCH_CORPUS"); c != "" {
		return c
	}
	return defaultCorpus
}

type scenario struct {
	Name     string
	Opts     GrepOptions
	RgArgs   []string
	MinCount int
}

func benchScenarios() []scenario {
	c := benchCorpus()
	return []scenario{
		{
			Name: "LiteralSimple",
			Opts: GrepOptions{
				Pattern: "error", Path: c, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "--type", "go", "error", c},
			MinCount: 1000,
		},
		{
			Name: "LiteralRare",
			Opts: GrepOptions{
				Pattern: "GODEBUG", Path: c,
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "GODEBUG", c},
			MinCount: 10,
		},
		{
			Name: "RegexFuncDef",
			Opts: GrepOptions{
				Pattern: `func \w+\(`, Path: c, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "--type", "go", `func \w+\(`, c},
			MinCount: 10000,
		},
		{
			Name: "CaseInsensitive",
			Opts: GrepOptions{
				Pattern: "error", Path: c, Type: "go",
				CaseInsensitive: true, OutputMode: "count",
				RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-ic", "--no-ignore", "--type", "go", "error", c},
			MinCount: 1000,
		},
		{
			Name: "Alternation",
			Opts: GrepOptions{
				Pattern: "error|warning|fatal|panic", Path: c, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "--type", "go", "error|warning|fatal|panic", c},
			MinCount: 1000,
		},
		{
			Name: "FilesWithMatches",
			Opts: GrepOptions{
				Pattern: "interface", Path: c, Type: "go",
				OutputMode:       "files_with_matches",
				RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-l", "--no-ignore", "--type", "go", "interface", c},
			MinCount: 100,
		},
		{
			Name: "CountHighFreq",
			Opts: GrepOptions{
				Pattern: "return", Path: c, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "--type", "go", "return", c},
			MinCount: 10000,
		},
		{
			Name: "NoTypeFilter",
			Opts: GrepOptions{
				Pattern: "TODO", Path: c,
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			RgArgs:   []string{"-c", "--no-ignore", "TODO", c},
			MinCount: 100,
		},
	}
}

// BenchmarkLib benchmarks the gogrep library directly (no subprocess overhead).
func BenchmarkLib(b *testing.B) {
	if _, err := os.Stat(benchCorpus()); err != nil {
		b.Skipf("corpus not found: %s", benchCorpus())
	}
	for _, sc := range benchScenarios() {
		b.Run(sc.Name, func(b *testing.B) {
			ctx := context.Background()
			// Warm up OS page cache
			if _, err := Grep(ctx, sc.Opts); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := Grep(ctx, sc.Opts)
				if err != nil {
					b.Fatal(err)
				}
				if results.TotalCount < sc.MinCount {
					b.Fatalf("expected >= %d matches, got %d", sc.MinCount, results.TotalCount)
				}
			}
		})
	}
}

// BenchmarkCLI_Rg benchmarks ripgrep as a subprocess.
func BenchmarkCLI_Rg(b *testing.B) {
	if _, err := os.Stat(rgBinary); err != nil {
		b.Skip("rg not found")
	}
	if _, err := os.Stat(benchCorpus()); err != nil {
		b.Skipf("corpus not found: %s", benchCorpus())
	}
	for _, sc := range benchScenarios() {
		b.Run(sc.Name, func(b *testing.B) {
			// Warm up
			exec.Command(rgBinary, sc.RgArgs...).Output()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cmd := exec.Command(rgBinary, sc.RgArgs...)
				if _, err := cmd.Output(); err != nil {
					if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
						b.Fatal("no matches found")
					}
				}
			}
		})
	}
}

// TestComparisonReport runs a side-by-side comparison of gogrep library vs rg CLI.
// Gated behind GOGREP_COMPARISON=1 to avoid running in normal test suites.
func TestComparisonReport(t *testing.T) {
	if os.Getenv("GOGREP_COMPARISON") == "" {
		t.Skip("set GOGREP_COMPARISON=1 to run comparison report")
	}
	if _, err := os.Stat(rgBinary); err != nil {
		t.Skip("rg not found")
	}
	if _, err := os.Stat(benchCorpus()); err != nil {
		t.Skipf("corpus not found: %s", benchCorpus())
	}

	iterations := 10
	if s := os.Getenv("GOGREP_ITERATIONS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			iterations = n
		}
	}

	fmt.Println()
	fmt.Println("=== gogrep vs rg Comparison Report ===")
	fmt.Printf("Corpus: %s\n", benchCorpus())
	fmt.Printf("Iterations: %d\n", iterations)
	fmt.Printf("rg version: %s\n", getRgVersion())
	fmt.Println()
	fmt.Printf("%-22s %12s %12s %8s\n",
		"Scenario", "gogrep(ms)", "rg(ms)", "Ratio")
	fmt.Println(strings.Repeat("-", 58))

	for _, sc := range benchScenarios() {
		ctx := context.Background()

		// Warm up
		exec.Command(rgBinary, sc.RgArgs...).Output()
		Grep(ctx, sc.Opts)

		// Measure rg
		rgTimes := make([]time.Duration, iterations)
		for i := range iterations {
			start := time.Now()
			cmd := exec.Command(rgBinary, sc.RgArgs...)
			cmd.Stdout = nil
			cmd.Stderr = nil
			cmd.Run()
			rgTimes[i] = time.Since(start)
		}

		// Measure gogrep library
		libTimes := make([]time.Duration, iterations)
		for i := range iterations {
			start := time.Now()
			Grep(ctx, sc.Opts)
			libTimes[i] = time.Since(start)
		}

		rgMed := medianDuration(rgTimes)
		libMed := medianDuration(libTimes)

		ratio := float64(libMed) / float64(rgMed)

		fmt.Printf("%-22s %12.1f %12.1f %7.2fx\n",
			sc.Name,
			float64(libMed.Microseconds())/1000.0,
			float64(rgMed.Microseconds())/1000.0,
			ratio,
		)
	}
	fmt.Println()
}

// TestParityWithRg verifies that gogrep produces identical results to rg.
// Runs against the real corpus to catch any discrepancies.
func TestParityWithRg(t *testing.T) {
	if _, err := os.Stat(rgBinary); err != nil {
		t.Skip("rg not found")
	}
	corpus := benchCorpus()
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("corpus not found: %s", corpus)
	}

	cases := []struct {
		name    string
		pattern string
		opts    GrepOptions
		rgArgs  []string
	}{
		{
			name:    "LiteralCount",
			pattern: "error",
			opts: GrepOptions{
				Pattern: "error", Path: corpus, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-c", "--no-ignore", "--type", "go", "error", corpus},
		},
		{
			name:    "CaseInsensitiveCount",
			pattern: "error",
			opts: GrepOptions{
				Pattern: "error", Path: corpus, Type: "go",
				CaseInsensitive: true, OutputMode: "count",
				RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-ic", "--no-ignore", "--type", "go", "error", corpus},
		},
		{
			name:    "RegexCount",
			pattern: `func \w+\(`,
			opts: GrepOptions{
				Pattern: `func \w+\(`, Path: corpus, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			// --no-unicode: Go's \w is ASCII-only [0-9A-Za-z_], rg's is Unicode by default
			rgArgs: []string{"-c", "--no-ignore", "--no-unicode", "--type", "go", `func \w+\(`, corpus},
		},
		{
			name:    "AlternationCount",
			pattern: "error|warning|fatal|panic",
			opts: GrepOptions{
				Pattern: "error|warning|fatal|panic", Path: corpus, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-c", "--no-ignore", "--type", "go", "error|warning|fatal|panic", corpus},
		},
		{
			name:    "FilesWithMatches",
			pattern: "interface",
			opts: GrepOptions{
				Pattern: "interface", Path: corpus, Type: "go",
				OutputMode: "files_with_matches", RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-l", "--no-ignore", "--type", "go", "interface", corpus},
		},
		{
			name:    "NoTypeFilter",
			pattern: "TODO",
			opts: GrepOptions{
				Pattern: "TODO", Path: corpus,
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-c", "--no-ignore", "TODO", corpus},
		},
		{
			name:    "RarePattern",
			pattern: "GODEBUG",
			opts: GrepOptions{
				Pattern: "GODEBUG", Path: corpus, Type: "go",
				OutputMode: "count", RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"-c", "--no-ignore", "--type", "go", "GODEBUG", corpus},
		},
		{
			name:    "ContentMode",
			pattern: "GODEBUG",
			opts: GrepOptions{
				Pattern: "GODEBUG", Path: corpus, Type: "go",
				RespectGitignore: boolPtr(false),
			},
			rgArgs: []string{"--no-ignore", "--type", "go", "--no-heading", "--no-line-number", "GODEBUG", corpus},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Run rg
			rgOut, err := exec.Command(rgBinary, tc.rgArgs...).Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
					t.Fatal("rg found no matches")
				}
			}

			// Run gogrep
			results, err := Grep(context.Background(), tc.opts)
			if err != nil {
				t.Fatalf("gogrep error: %v", err)
			}

			switch tc.opts.OutputMode {
			case "count":
				// Parse rg count output: "path:N" per line, sum N values
				rgTotal := parseRgCountTotal(string(rgOut))
				if results.TotalCount != rgTotal {
					t.Errorf("count mismatch for %q: gogrep=%d, rg=%d",
						tc.pattern, results.TotalCount, rgTotal)
				}

				// Also verify per-file counts match
				rgFileCounts := parseRgFileCounts(string(rgOut))
				goFileCounts := make(map[string]int)
				for _, f := range results.Files {
					goFileCounts[f.Path] = f.Count
				}

				for path, rgCount := range rgFileCounts {
					goCount, ok := goFileCounts[path]
					if !ok {
						t.Errorf("rg matched file %s (%d matches) but gogrep did not", path, rgCount)
						continue
					}
					if goCount != rgCount {
						t.Errorf("file %s: gogrep=%d, rg=%d", path, goCount, rgCount)
					}
				}
				for path, goCount := range goFileCounts {
					if _, ok := rgFileCounts[path]; !ok {
						t.Errorf("gogrep matched file %s (%d matches) but rg did not", path, goCount)
					}
				}

			case "files_with_matches":
				rgFiles := parseRgFileList(string(rgOut))
				goFiles := make(map[string]bool)
				for _, f := range results.Files {
					goFiles[f.Path] = true
				}

				if len(rgFiles) != len(goFiles) {
					t.Errorf("file count mismatch: gogrep=%d, rg=%d", len(goFiles), len(rgFiles))
				}
				for path := range rgFiles {
					if !goFiles[path] {
						t.Errorf("rg matched %s but gogrep did not", path)
					}
				}
				for path := range goFiles {
					if !rgFiles[path] {
						t.Errorf("gogrep matched %s but rg did not", path)
					}
				}

			default: // content mode
				// Compare matched lines: rg outputs "path:line_content"
				rgLines := parseRgContentLines(string(rgOut))
				var goLines []string
				for _, f := range results.Files {
					for _, m := range f.Matches {
						goLines = append(goLines, m.Path+":"+m.Line)
					}
				}

				slices.Sort(rgLines)
				slices.Sort(goLines)

				if len(rgLines) != len(goLines) {
					t.Errorf("match count mismatch: gogrep=%d lines, rg=%d lines",
						len(goLines), len(rgLines))
					// Show first few diffs
					showDiffs(t, goLines, rgLines, 10)
				} else {
					mismatches := 0
					for i := range rgLines {
						if rgLines[i] != goLines[i] {
							if mismatches < 10 {
								t.Errorf("line %d differs:\n  rg:     %s\n  gogrep: %s", i, rgLines[i], goLines[i])
							}
							mismatches++
						}
					}
					if mismatches > 10 {
						t.Errorf("... and %d more mismatches", mismatches-10)
					}
				}
			}
		})
	}
}

func parseRgCountTotal(output string) int {
	total := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		n, err := strconv.Atoi(line[idx+1:])
		if err == nil {
			total += n
		}
	}
	return total
}

func parseRgFileCounts(output string) map[string]int {
	counts := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		path := line[:idx]
		n, err := strconv.Atoi(line[idx+1:])
		if err == nil {
			counts[path] = n
		}
	}
	return counts
}

func parseRgFileList(output string) map[string]bool {
	files := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files[line] = true
		}
	}
	return files
}

func parseRgContentLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func showDiffs(t *testing.T, goLines, rgLines []string, maxDiffs int) {
	t.Helper()
	goSet := make(map[string]bool)
	for _, l := range goLines {
		goSet[l] = true
	}
	rgSet := make(map[string]bool)
	for _, l := range rgLines {
		rgSet[l] = true
	}
	shown := 0
	for _, l := range rgLines {
		if !goSet[l] && shown < maxDiffs {
			t.Errorf("  in rg but not gogrep: %s", l)
			shown++
		}
	}
	for _, l := range goLines {
		if !rgSet[l] && shown < maxDiffs {
			t.Errorf("  in gogrep but not rg: %s", l)
			shown++
		}
	}
}

func medianDuration(d []time.Duration) time.Duration {
	sorted := make([]time.Duration, len(d))
	copy(sorted, d)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}

func getRgVersion() string {
	out, err := exec.Command(rgBinary, "--version").Output()
	if err != nil {
		return "unknown"
	}
	lines := strings.SplitN(string(out), "\n", 2)
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return "unknown"
}
