package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPatternMatching(t *testing.T) {
	// Create temp directory structure
	dir := t.TempDir()

	// Initialize a git repo
	gitDir := filepath.Join(dir, ".git")
	os.Mkdir(gitDir, 0o755)

	// Create .gitignore
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(
		"*.log\nbuild/\n!important.log\ntemp*\n#comment\n\n*.tmp\n"), 0o644)

	m := New(dir)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"debug.log", false, true},
		{"src/debug.log", false, true},
		{"important.log", false, false}, // negated
		{"main.go", false, false},
		{"build", true, true},   // dir-only pattern
		{"build", false, false}, // not a dir
		{"tempfile", false, true},
		{"temporary", false, true},
		{"data.tmp", false, true},
		{"src/data.tmp", false, true},
		{"readme.md", false, false},
	}

	for _, tt := range tests {
		absPath := filepath.Join(dir, tt.path)
		got := m.IsIgnored(absPath, tt.isDir)
		if got != tt.want {
			t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestNestedGitignore(t *testing.T) {
	dir := t.TempDir()

	// Initialize git repo
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)

	// Root .gitignore
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644)

	// Create subdirectory with its own .gitignore
	subDir := filepath.Join(dir, "src")
	os.Mkdir(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte("*.generated.go\n"), 0o644)

	m := New(dir)
	m.LoadDir(subDir)

	// Root pattern should work
	if !m.IsIgnored(filepath.Join(dir, "app.log"), false) {
		t.Error("expected app.log to be ignored by root .gitignore")
	}

	// Nested pattern should work
	if !m.IsIgnored(filepath.Join(subDir, "code.generated.go"), false) {
		t.Error("expected code.generated.go to be ignored by nested .gitignore")
	}

	// Other files should not be ignored
	if m.IsIgnored(filepath.Join(subDir, "main.go"), false) {
		t.Error("expected main.go to not be ignored")
	}
}

func TestDoublestarPattern(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("**/test/*.txt\ndocs/**\n"), 0o644)

	m := New(dir)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"test/file.txt", false, true},
		{"a/test/file.txt", false, true},
		{"a/b/test/file.txt", false, true},
		{"test/file.go", false, false},
		{"docs/readme.md", false, true},
		{"docs/api/ref.md", false, true},
	}

	for _, tt := range tests {
		absPath := filepath.Join(dir, tt.path)
		got := m.IsIgnored(absPath, tt.isDir)
		if got != tt.want {
			t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestAnchoredPattern(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/TODO\nsrc/vendor/\n"), 0o644)

	m := New(dir)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"TODO", false, true},
		{"src/TODO", false, false}, // anchored, doesn't match nested
		{"src/vendor", true, true},
		{"src/vendor", false, false}, // dir-only
	}

	for _, tt := range tests {
		absPath := filepath.Join(dir, tt.path)
		got := m.IsIgnored(absPath, tt.isDir)
		if got != tt.want {
			t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestNoGitRepo(t *testing.T) {
	dir := t.TempDir()
	// No .git directory

	m := New(dir)

	// Nothing should be ignored
	if m.IsIgnored(filepath.Join(dir, "anything.log"), false) {
		t.Error("expected nothing to be ignored outside git repo")
	}
}

// ============================================================
// Negation precedence (ripgrep: gitignore.rs ig7, ignot6)
// ============================================================

func TestNegationPrecedence(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	// Ignore all .rs files, then un-ignore main.rs
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.rs\n!main.rs\n"), 0o644)

	m := New(dir)

	if m.IsIgnored(filepath.Join(dir, "main.rs"), false) {
		t.Error("main.rs should NOT be ignored (negation pattern)")
	}
	if !m.IsIgnored(filepath.Join(dir, "lib.rs"), false) {
		t.Error("lib.rs SHOULD be ignored")
	}
	if !m.IsIgnored(filepath.Join(dir, "other.rs"), false) {
		t.Error("other.rs SHOULD be ignored")
	}
}

func TestNegationThenMatch(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	// Un-ignore first, then ignore — last match wins
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("!src/main.rs\n*.rs\n"), 0o644)

	m := New(dir)

	// Last pattern wins: *.rs overrides !src/main.rs
	if !m.IsIgnored(filepath.Join(dir, "src/main.rs"), false) {
		t.Error("src/main.rs should be ignored (last pattern wins)")
	}
}

// ============================================================
// Comment and blank line handling (ripgrep: gitignore.rs ignot11, ignot12)
// ============================================================

func TestCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(
		"# This is a comment\n\n\n*.log\n# Another comment\n\n*.tmp\n"), 0o644)

	m := New(dir)

	if m.IsIgnored(filepath.Join(dir, "# This is a comment"), false) {
		t.Error("comment lines should not be treated as patterns")
	}
	if !m.IsIgnored(filepath.Join(dir, "debug.log"), false) {
		t.Error("*.log should still work after comments")
	}
	if !m.IsIgnored(filepath.Join(dir, "temp.tmp"), false) {
		t.Error("*.tmp should still work after blank lines")
	}
}

// ============================================================
// Escaped metacharacters (ripgrep: gitignore.rs ig21, ig38-41)
// ============================================================

func TestEscapedPatterns(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	// Escaped ! should match literal !
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("\\!important\n\\#foo\n"), 0o644)

	m := New(dir)

	if !m.IsIgnored(filepath.Join(dir, "!important"), false) {
		t.Error("\\!important should match literal !important")
	}
	if !m.IsIgnored(filepath.Join(dir, "#foo"), false) {
		t.Error("\\#foo should match literal #foo")
	}
}

// ============================================================
// Doublestar edge cases (ripgrep: gitignore.rs ig9-20)
// ============================================================

func TestDoublestarEdgeCases(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(
		"**/foo\na/**/b\nbar/**\n"), 0o644)

	m := New(dir)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		// **/foo matches foo anywhere
		{"foo", false, true},
		{"a/foo", false, true},
		{"a/b/c/foo", false, true},
		// a/**/b matches a/b, a/x/b, a/x/y/b
		{"a/b", false, true},
		{"a/x/b", false, true},
		{"a/x/y/b", false, true},
		// bar/** matches everything inside bar
		{"bar/x", false, true},
		{"bar/x/y", false, true},
		// Should not match
		{"baz", false, false},
		{"foobar", false, false},
	}

	for _, tt := range tests {
		absPath := filepath.Join(dir, tt.path)
		got := m.IsIgnored(absPath, tt.isDir)
		if got != tt.want {
			t.Errorf("IsIgnored(%q, isDir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

// ============================================================
// Trailing whitespace in patterns (ripgrep: gitignore.rs)
// ============================================================

func TestTrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	// Pattern with trailing spaces should have them stripped
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log   \n*.tmp\t\t\n"), 0o644)

	m := New(dir)

	if !m.IsIgnored(filepath.Join(dir, "debug.log"), false) {
		t.Error("*.log (with trailing spaces stripped) should match")
	}
	if !m.IsIgnored(filepath.Join(dir, "temp.tmp"), false) {
		t.Error("*.tmp (with trailing tabs stripped) should match")
	}
}

// ============================================================
// Concurrent safety
// ============================================================

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.log\n"), 0o644)

	m := New(dir)

	// Hammer IsIgnored and LoadDir concurrently
	done := make(chan struct{})
	for range 10 {
		go func() {
			for range 100 {
				m.IsIgnored(filepath.Join(dir, "test.log"), false)
				m.IsIgnored(filepath.Join(dir, "test.go"), false)
			}
			done <- struct{}{}
		}()
	}
	for range 10 {
		go func() {
			for range 10 {
				m.LoadDir(dir)
			}
			done <- struct{}{}
		}()
	}
	for range 20 {
		<-done
	}
}

// ============================================================
// Ripgrep gitignore.rs: ig1-ig44 (should be ignored)
// Ported from crates/ignore/src/gitignore.rs
// ============================================================

func TestRipgrepIgnored(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		isDir   bool
	}{
		{"ig1", "months", "months", false},
		{"ig2", "*.lock", "Cargo.lock", false},
		{"ig3", "*.rs", "src/main.rs", false},
		{"ig4", "src/*.rs", "src/main.rs", false},
		{"ig5", "/*.c", "cat-file.c", false},
		{"ig6", "/src/*.rs", "src/main.rs", false},
		{"ig7", "!src/main.rs\n*.rs", "src/main.rs", false}, // last pattern wins
		{"ig8", "foo/", "foo", true},
		{"ig9", "**/foo", "foo", false},
		{"ig10", "**/foo", "src/foo", false},
		{"ig11", "**/foo/**", "src/foo/bar", false},
		{"ig12", "**/foo/**", "wat/src/foo/bar/baz", false},
		{"ig13", "**/foo/bar", "foo/bar", false},
		{"ig14", "**/foo/bar", "src/foo/bar", false},
		{"ig15", "abc/**", "abc/x", false},
		{"ig16", "abc/**", "abc/x/y", false},
		{"ig17", "abc/**", "abc/x/y/z", false},
		{"ig18", "a/**/b", "a/b", false},
		{"ig19", "a/**/b", "a/x/b", false},
		{"ig20", "a/**/b", "a/x/y/b", false},
		{"ig21", "\\!xy", "!xy", false},
		{"ig22", "\\#foo", "#foo", false},
		{"ig23", "foo", "foo", false},
		{"ig24", "target", "grep/target", false},
		{"ig25", "Cargo.lock", "tabwriter-bin/Cargo.lock", false},
		{"ig26", "/foo/bar/baz", "foo/bar/baz", false},
		{"ig27", "foo/", "xyz/foo", true},
		{"ig29", "node_modules/ ", "node_modules", true}, // trailing space stripped
		{"ig30", "**/", "foo/bar", true},
		{"ig31", "path1/*", "path1/foo", false},
		{"ig32", ".a/b", ".a/b", false},
		{"ig38", "\\[", "[", false},
		{"ig39", "\\?", "?", false},
		{"ig40", "\\*", "*", false},
		{"ig41", "\\a", "a", false},
		{"ig42", "s*.rs", "sfoo.rs", false},
		{"ig43", "**", "foo.rs", false},
		{"ig44", "**/**/*", "a/foo.rs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
			os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(tt.pattern+"\n"), 0o644)
			m := New(dir)
			absPath := filepath.Join(dir, tt.path)
			if !m.IsIgnored(absPath, tt.isDir) {
				t.Errorf("expected %q to be ignored with pattern %q", tt.path, tt.pattern)
			}
		})
	}
}

// ============================================================
// Ripgrep gitignore.rs: ignot1-ignot19 (should NOT be ignored)
// Ported from crates/ignore/src/gitignore.rs
// ============================================================

func TestRipgrepNotIgnored(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		isDir   bool
	}{
		{"ignot1", "amonths", "months", false},
		{"ignot2", "monthsa", "months", false},
		{"ignot3", "/src/*.rs", "src/grep/src/main.rs", false},
		{"ignot4", "/*.c", "mozilla-sha1/sha1.c", false},
		{"ignot5", "/src/*.rs", "src/grep/src/main.rs", false},
		{"ignot6", "*.rs\n!src/main.rs", "src/main.rs", false}, // negation wins
		{"ignot7", "foo/", "foo", false},                       // dir-only, not a dir
		{"ignot8", "**/foo/**", "wat/src/afoo/bar/baz", false},
		{"ignot9", "**/foo/**", "wat/src/fooa/bar/baz", false},
		{"ignot10", "**/foo/bar", "foo/src/bar", false},
		{"ignot11", "#foo", "#foo", false},     // comment line
		{"ignot12", "\n\n\n", "foo", false},    // blank lines only
		{"ignot13", "foo/**", "foo", true},     // foo/** doesn't match foo itself
		{"ignot15", "!/bar", "foo/bar", false}, // negation of anchored pattern
		{"ignot16", "*\n!**/", "foo", true},    // negated dir pattern
		{"ignot17", "src/*.rs", "src/grep/src/main.rs", false},
		{"ignot18", "path1/*", "path2/path1/foo", false},
		{"ignot19", "s*.rs", "src/foo.rs", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
			os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(tt.pattern+"\n"), 0o644)
			m := New(dir)
			absPath := filepath.Join(dir, tt.path)
			if m.IsIgnored(absPath, tt.isDir) {
				t.Errorf("expected %q to NOT be ignored with pattern %q", tt.path, tt.pattern)
			}
		})
	}
}

// ============================================================
// Ripgrep gitignore.rs: ig28 (subdirectory root)
// ============================================================

func TestRipgrepIgnoredSubdirRoot(t *testing.T) {
	// ig28: root="./src", pattern="/llvm/", path="./src/llvm" as dir
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0o755)
	os.WriteFile(filepath.Join(srcDir, ".gitignore"), []byte("/llvm/\n"), 0o644)

	m := New(srcDir)
	absPath := filepath.Join(srcDir, "llvm")
	if !m.IsIgnored(absPath, true) {
		t.Error("ig28: expected llvm dir to be ignored with /llvm/ pattern from src root")
	}
}

// ============================================================
// Ripgrep gitignore.rs: ignot14 (subdirectory root, no match)
// ============================================================

func TestRipgrepNotIgnoredSubdirRoot(t *testing.T) {
	// ignot14: root="./third_party/protobuf", pattern="m4/ltoptions.m4",
	// path shouldn't match unrelated file
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	subDir := filepath.Join(dir, "third_party", "protobuf")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte("m4/ltoptions.m4\n"), 0o644)

	m := New(subDir)
	absPath := filepath.Join(subDir, "csharp", "src", "packages", "repositories.config")
	if m.IsIgnored(absPath, false) {
		t.Error("ignot14: expected repositories.config to NOT be ignored")
	}
}

// ============================================================
// Ripgrep gitignore.rs: case sensitivity
// ============================================================

func TestCaseSensitive(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.html\n"), 0o644)

	m := New(dir)

	// cs1: *.html should match foo.html
	if !m.IsIgnored(filepath.Join(dir, "foo.html"), false) {
		t.Error("cs1: expected foo.html to be ignored by *.html")
	}
	// cs2: *.html should NOT match foo.HTML (case-sensitive)
	if m.IsIgnored(filepath.Join(dir, "foo.HTML"), false) {
		t.Error("cs2: expected foo.HTML to NOT be ignored by *.html (case-sensitive)")
	}
	// cs3: *.html should NOT match foo.htm
	if m.IsIgnored(filepath.Join(dir, "foo.htm"), false) {
		t.Error("cs3: expected foo.htm to NOT be ignored by *.html")
	}
}
