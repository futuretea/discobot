package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// SkillMarketRepo manages a single git clone of a skill market repository.
// It is safe for concurrent use.
type SkillMarketRepo struct {
	mu         sync.Mutex
	cloneDir   string // absolute path to the local clone root
	repoURL    string
	branch     string
	skillsPath string // relative path inside the repo where skill sub-directories live
}

// EnsureCloned ensures the repository is available locally.
//
// If the clone does not yet exist, it performs a fresh "git clone --depth=1".
// If the clone already exists and forceUpdate is true, it runs "git pull --ff-only"
// to fetch the latest changes.  When forceUpdate is false the existing clone is
// used as-is (fast path — no network I/O).
func (r *SkillMarketRepo) EnsureCloned(ctx context.Context, forceUpdate bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, statErr := os.Stat(filepath.Join(r.cloneDir, ".git"))
	alreadyCloned := statErr == nil

	if alreadyCloned {
		if !forceUpdate {
			// Serve from the existing local clone — no network call.
			return nil
		}
		// Pull latest changes on explicit reload.
		log.Printf("[SkillMarketRepo] pulling %s (branch %s) into %s", r.repoURL, r.branch, r.cloneDir)
		cmd := exec.CommandContext(ctx, "git", "-C", r.cloneDir, "pull", "--ff-only", "--depth=1", "origin", r.branch)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			// Non-fatal: serve from the existing (possibly stale) clone.
			log.Printf("[SkillMarketRepo] git pull failed (serving stale cache): %v", err)
		}
		return nil
	}

	// Fresh clone.
	if err := os.MkdirAll(r.cloneDir, 0o755); err != nil {
		return fmt.Errorf("failed to create clone directory %s: %w", r.cloneDir, err)
	}
	log.Printf("[SkillMarketRepo] cloning %s (branch %s) into %s", r.repoURL, r.branch, r.cloneDir)
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--branch", r.branch, r.repoURL, r.cloneDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Clean up the partial clone so the next attempt retries from scratch.
		_ = os.RemoveAll(r.cloneDir)
		return fmt.Errorf("git clone failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// SkillsDir returns the absolute path to the directory inside the clone that
// contains skill sub-directories (one sub-directory = one skill).
func (r *SkillMarketRepo) SkillsDir() string {
	return filepath.Join(r.cloneDir, r.skillsPath)
}

// CopySkillDir copies the skill sub-directory named skillSubdir from the clone
// into dstDir, creating dstDir if it does not exist.
func (r *SkillMarketRepo) CopySkillDir(skillSubdir, dstDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	src := filepath.Join(r.SkillsDir(), skillSubdir)
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("skill subdirectory %q not found in clone", skillSubdir)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	// os.CopyFS copies the entire tree rooted at src into dstDir (Go 1.23+).
	return os.CopyFS(dstDir, os.DirFS(src))
}

// ────────────────────────────────────────────────────────────────────────────
// SkillMarketCache — manages multiple SkillMarketRepo instances keyed by
// "repoURL@branch".
// ────────────────────────────────────────────────────────────────────────────

// SkillMarketCache holds a pool of SkillMarketRepo instances, one per unique
// (repoURL, branch) pair.  It is safe for concurrent use.
type SkillMarketCache struct {
	mu           sync.RWMutex
	repos        map[string]*SkillMarketRepo
	cacheBaseDir string
}

// NewSkillMarketCache creates a new cache that stores git clones under cacheBaseDir.
func NewSkillMarketCache(cacheBaseDir string) *SkillMarketCache {
	return &SkillMarketCache{
		repos:        make(map[string]*SkillMarketRepo),
		cacheBaseDir: cacheBaseDir,
	}
}

// Get returns (creating if necessary) the SkillMarketRepo for the given
// (repoURL, branch, skillsPath) triple.  The clone directory is derived from a
// hash of the repoURL+branch so that each unique repo+branch gets its own
// sub-directory under cacheBaseDir.
func (c *SkillMarketCache) Get(repoURL, branch, skillsPath string) *SkillMarketRepo {
	key := repoURL + "@" + branch

	c.mu.RLock()
	if r, ok := c.repos[key]; ok {
		c.mu.RUnlock()
		return r
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if r, ok := c.repos[key]; ok {
		return r
	}

	// Derive a stable sub-directory name from a hash of the key.
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))[:16]
	cloneDir := filepath.Join(c.cacheBaseDir, hash)

	r := &SkillMarketRepo{
		cloneDir:   cloneDir,
		repoURL:    repoURL,
		branch:     branch,
		skillsPath: skillsPath,
	}
	c.repos[key] = r
	return r
}
