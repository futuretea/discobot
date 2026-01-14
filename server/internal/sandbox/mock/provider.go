// Package mock provides a mock implementation of sandbox.Provider for testing.
package mock

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/anthropics/octobot/server/internal/sandbox"
)

// Provider is a mock sandbox provider for testing.
type Provider struct {
	mu        sync.RWMutex
	sandboxes map[string]*sandbox.Sandbox

	// Configurable behaviors for testing
	CreateFunc func(ctx context.Context, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, error)
	StartFunc  func(ctx context.Context, sessionID string) error
	StopFunc   func(ctx context.Context, sessionID string, timeout time.Duration) error
	RemoveFunc func(ctx context.Context, sessionID string) error
	GetFunc    func(ctx context.Context, sessionID string) (*sandbox.Sandbox, error)
	ExecFunc   func(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecOptions) (*sandbox.ExecResult, error)
	AttachFunc func(ctx context.Context, sessionID string, opts sandbox.AttachOptions) (sandbox.PTY, error)
}

// NewProvider creates a new mock provider with default behavior.
func NewProvider() *Provider {
	return &Provider{
		sandboxes: make(map[string]*sandbox.Sandbox),
	}
}

// Create creates a mock sandbox.
func (p *Provider) Create(ctx context.Context, sessionID string, opts sandbox.CreateOptions) (*sandbox.Sandbox, error) {
	if p.CreateFunc != nil {
		return p.CreateFunc(ctx, sessionID, opts)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.sandboxes[sessionID]; exists {
		return nil, sandbox.ErrAlreadyExists
	}

	// Simulate port assignments for requested port mappings
	var ports []sandbox.AssignedPort
	for _, pm := range opts.Ports {
		protocol := pm.Protocol
		if protocol == "" {
			protocol = "tcp"
		}
		// Simulate random port assignment (using a deterministic value for testing)
		hostPort := pm.HostPort
		if hostPort == 0 {
			hostPort = 32768 + pm.ContainerPort // Predictable for testing
		}
		ports = append(ports, sandbox.AssignedPort{
			ContainerPort: pm.ContainerPort,
			HostPort:      hostPort,
			HostIP:        "0.0.0.0",
			Protocol:      protocol,
		})
	}

	s := &sandbox.Sandbox{
		ID:        "mock-" + sessionID,
		SessionID: sessionID,
		Status:    sandbox.StatusCreated,
		Image:     opts.Image,
		CreatedAt: time.Now(),
		Metadata:  map[string]string{"mock": "true"},
		Ports:     ports,
		Env:       opts.Env,
	}
	p.sandboxes[sessionID] = s
	return s, nil
}

// Start starts a mock sandbox.
func (p *Provider) Start(ctx context.Context, sessionID string) error {
	if p.StartFunc != nil {
		return p.StartFunc(ctx, sessionID)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	s, exists := p.sandboxes[sessionID]
	if !exists {
		return sandbox.ErrNotFound
	}

	if s.Status == sandbox.StatusRunning {
		return sandbox.ErrAlreadyRunning
	}

	s.Status = sandbox.StatusRunning
	now := time.Now()
	s.StartedAt = &now
	return nil
}

// Stop stops a mock sandbox.
func (p *Provider) Stop(ctx context.Context, sessionID string, timeout time.Duration) error {
	if p.StopFunc != nil {
		return p.StopFunc(ctx, sessionID, timeout)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	s, exists := p.sandboxes[sessionID]
	if !exists {
		return sandbox.ErrNotFound
	}

	if s.Status != sandbox.StatusRunning {
		return sandbox.ErrNotRunning
	}

	s.Status = sandbox.StatusStopped
	now := time.Now()
	s.StoppedAt = &now
	return nil
}

// Remove removes a mock sandbox.
func (p *Provider) Remove(ctx context.Context, sessionID string) error {
	if p.RemoveFunc != nil {
		return p.RemoveFunc(ctx, sessionID)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.sandboxes[sessionID]; !exists {
		return nil // Idempotent
	}

	delete(p.sandboxes, sessionID)
	return nil
}

// Get returns a mock sandbox.
func (p *Provider) Get(ctx context.Context, sessionID string) (*sandbox.Sandbox, error) {
	if p.GetFunc != nil {
		return p.GetFunc(ctx, sessionID)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	s, exists := p.sandboxes[sessionID]
	if !exists {
		return nil, sandbox.ErrNotFound
	}

	// Return a copy
	cpy := *s
	return &cpy, nil
}

// Exec runs a mock command.
func (p *Provider) Exec(ctx context.Context, sessionID string, cmd []string, opts sandbox.ExecOptions) (*sandbox.ExecResult, error) {
	if p.ExecFunc != nil {
		return p.ExecFunc(ctx, sessionID, cmd, opts)
	}

	p.mu.RLock()
	_, exists := p.sandboxes[sessionID]
	p.mu.RUnlock()

	if !exists {
		return nil, sandbox.ErrNotFound
	}

	return &sandbox.ExecResult{
		ExitCode: 0,
		Stdout:   []byte("mock output\n"),
		Stderr:   []byte{},
	}, nil
}

// Attach creates a mock PTY.
func (p *Provider) Attach(ctx context.Context, sessionID string, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	if p.AttachFunc != nil {
		return p.AttachFunc(ctx, sessionID, opts)
	}

	p.mu.RLock()
	s, exists := p.sandboxes[sessionID]
	p.mu.RUnlock()

	if !exists {
		return nil, sandbox.ErrNotFound
	}

	if s.Status != sandbox.StatusRunning {
		return nil, sandbox.ErrNotRunning
	}

	return &MockPTY{}, nil
}

// List returns all sandboxes managed by this mock provider.
func (p *Provider) List(ctx context.Context) ([]*sandbox.Sandbox, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*sandbox.Sandbox, 0, len(p.sandboxes))
	for _, v := range p.sandboxes {
		cpy := *v
		result = append(result, &cpy)
	}
	return result, nil
}

// GetSandboxes returns all sandboxes (for test assertions).
func (p *Provider) GetSandboxes() map[string]*sandbox.Sandbox {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*sandbox.Sandbox)
	for k, v := range p.sandboxes {
		cpy := *v
		result[k] = &cpy
	}
	return result
}

// MockPTY is a mock PTY for testing.
type MockPTY struct {
	InputBuffer  []byte
	OutputBuffer []byte
	Closed       bool
	ResizeCalls  []struct{ Rows, Cols int }
	mu           sync.Mutex
}

func (p *MockPTY) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Closed {
		return 0, io.EOF
	}

	if len(p.OutputBuffer) == 0 {
		// Simulate some output
		p.OutputBuffer = []byte("$ ")
	}

	n := copy(b, p.OutputBuffer)
	p.OutputBuffer = p.OutputBuffer[n:]
	return n, nil
}

func (p *MockPTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Closed {
		return 0, io.ErrClosedPipe
	}

	p.InputBuffer = append(p.InputBuffer, b...)
	// Echo input to output
	p.OutputBuffer = append(p.OutputBuffer, b...)
	return len(b), nil
}

func (p *MockPTY) Resize(ctx context.Context, rows, cols int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ResizeCalls = append(p.ResizeCalls, struct{ Rows, Cols int }{rows, cols})
	return nil
}

func (p *MockPTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Closed = true
	return nil
}

func (p *MockPTY) Wait(ctx context.Context) (int, error) {
	return 0, nil
}
