package gogrep

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupBenchDir creates a directory with many files for benchmarking.
func setupBenchDir(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()

	// Create 100 Go files with realistic content
	for i := range 100 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("package pkg%d\n\n", i))
		sb.WriteString("import (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
		for j := range 50 {
			sb.WriteString(fmt.Sprintf("func Function%d_%d(input string) string {\n", i, j))
			sb.WriteString(fmt.Sprintf("\tresult := strings.Replace(input, \"old%d\", \"new%d\", -1)\n", j, j))
			sb.WriteString(fmt.Sprintf("\tfmt.Println(\"processing item %d\")\n", j))
			sb.WriteString("\treturn result\n}\n\n")
		}
		path := filepath.Join(dir, fmt.Sprintf("file_%03d.go", i))
		if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	// Create some non-Go files too
	for i := range 20 {
		path := filepath.Join(dir, fmt.Sprintf("data_%03d.json", i))
		content := fmt.Sprintf(`{"id": %d, "name": "item_%d", "value": %d}`, i, i, i*42)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}

	return dir
}

func BenchmarkGrepLiteral(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern: "processing item",
		Path:    dir,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if results.TotalCount == 0 {
			b.Fatal("expected matches")
		}
	}
}

func BenchmarkGrepRegex(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern: `Function\d+_\d+`,
		Path:    dir,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if results.TotalCount == 0 {
			b.Fatal("expected matches")
		}
	}
}

func BenchmarkGrepAlternation(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern: "old10|old20|old30|old40",
		Path:    dir,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if results.TotalCount == 0 {
			b.Fatal("expected matches")
		}
	}
}

func BenchmarkGrepFileType(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern: "Function",
		Path:    dir,
		Type:    "go",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if results.TotalCount == 0 {
			b.Fatal("expected matches")
		}
	}
}

func BenchmarkGrepCaseInsensitive(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern:         "function",
		Path:            dir,
		CaseInsensitive: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if results.TotalCount == 0 {
			b.Fatal("expected matches")
		}
	}
}

func BenchmarkGrepFilesWithMatches(b *testing.B) {
	dir := setupBenchDir(b)
	ctx := context.Background()
	opts := GrepOptions{
		Pattern:    "Function",
		Path:       dir,
		OutputMode: "files_with_matches",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := Grep(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
		if len(results.Files) == 0 {
			b.Fatal("expected matches")
		}
	}
}
