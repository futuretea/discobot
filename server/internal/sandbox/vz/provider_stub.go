// Package vz provides a macOS Virtualization.framework-based implementation of the sandbox.Provider interface.
// This stub file is used on non-darwin platforms where the vz library is not available.
//go:build !darwin

package vz

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/anthropics/octobot/server/internal/config"
	"github.com/anthropics/octobot/server/internal/sandbox"
)

// Config holds vz-specific configuration.
type Config struct {
	DataDir      string
	KernelPath   string
	InitrdPath   string
	BaseDiskPath string
}

// Provider is a stub that returns an error on non-darwin platforms.
type Provider struct{}

// NewProvider returns an error on non-darwin platforms.
func NewProvider(cfg *config.Config, vzCfg *Config) (*Provider, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS (darwin), current platform: %s", runtime.GOOS)
}

// ImageExists always returns false on non-darwin platforms.
func (p *Provider) ImageExists(ctx context.Context) bool {
	return false
}

// Image returns empty string on non-darwin platforms.
func (p *Provider) Image() string {
	return ""
}

// Create returns an error on non-darwin platforms.
func (p *Provider) Create(ctx context.Context, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Start returns an error on non-darwin platforms.
func (p *Provider) Start(ctx context.Context, sessionID string) error {
	return fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Stop returns an error on non-darwin platforms.
func (p *Provider) Stop(ctx context.Context, sessionID string, timeout time.Duration) error {
	return fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Remove returns an error on non-darwin platforms.
func (p *Provider) Remove(ctx context.Context, sessionID string) error {
	return fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Get returns an error on non-darwin platforms.
func (p *Provider) Get(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// GetSecret returns an error on non-darwin platforms.
func (p *Provider) GetSecret(ctx context.Context, sessionID string) (string, error) {
	return "", fmt.Errorf("vz sandbox provider is only available on macOS")
}

// List returns an error on non-darwin platforms.
func (p *Provider) List(ctx context.Context) ([]*sandbox.Sandbox, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Exec returns an error on non-darwin platforms.
func (p *Provider) Exec(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecOptions) (*sandbox.ExecResult, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Attach returns an error on non-darwin platforms.
func (p *Provider) Attach(ctx context.Context, sessionID string, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// HTTPClient returns an error on non-darwin platforms.
func (p *Provider) HTTPClient(ctx context.Context, sessionID string) (*http.Client, error) {
	return nil, fmt.Errorf("vz sandbox provider is only available on macOS")
}

// Close is a no-op on non-darwin platforms.
func (p *Provider) Close() error {
	return nil
}
