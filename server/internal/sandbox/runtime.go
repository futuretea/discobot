// Package sandbox provides an abstraction for sandbox execution environments.
// It supports multiple backends including Docker, Kubernetes, and Cloudflare sandboxes.
package sandbox

import (
	"context"
	"io"
	"time"
)

// Provider abstracts sandbox execution environments (Docker, K8s, Cloudflare, etc.)
// Each session gets one dedicated sandbox, managed through this interface.
type Provider interface {
	// Create creates a new sandbox for the given session.
	// The sandbox is created but not started.
	Create(ctx context.Context, sessionID string, opts CreateOptions) (*Sandbox, error)

	// Start starts a previously created sandbox.
	Start(ctx context.Context, sessionID string) error

	// Stop stops a running sandbox gracefully.
	// The timeout specifies how long to wait before force-killing.
	Stop(ctx context.Context, sessionID string, timeout time.Duration) error

	// Remove removes a sandbox and its resources.
	// The sandbox must be stopped first.
	Remove(ctx context.Context, sessionID string) error

	// Get returns the current state of a sandbox.
	Get(ctx context.Context, sessionID string) (*Sandbox, error)

	// List returns all sandboxes managed by octobot.
	// This includes sandboxes in any state (running, stopped, failed).
	List(ctx context.Context) ([]*Sandbox, error)

	// Exec runs a non-interactive command in the sandbox.
	// Returns stdout, stderr, and exit code.
	Exec(ctx context.Context, sessionID string, cmd []string, opts ExecOptions) (*ExecResult, error)

	// Attach creates an interactive PTY session to the sandbox.
	// The PTY can be used for bidirectional terminal communication.
	Attach(ctx context.Context, sessionID string, opts AttachOptions) (PTY, error)
}

// Sandbox represents a running or stopped sandbox instance.
type Sandbox struct {
	ID        string            // Runtime-specific sandbox ID
	SessionID string            // Octobot session ID (1:1 mapping)
	Status    SandboxStatus     // created, running, stopped, failed
	Image     string            // Sandbox image used
	CreatedAt time.Time         // When the sandbox was created
	StartedAt *time.Time        // When the sandbox was started (nil if never started)
	StoppedAt *time.Time        // When the sandbox was stopped (nil if still running)
	Error     string            // Error message if status == failed
	Metadata  map[string]string // Runtime-specific metadata
	Ports     []AssignedPort    // Assigned port mappings after sandbox creation
	Env       map[string]string // Environment variables set on the sandbox
}

// AssignedPort represents a port mapping that was assigned after sandbox creation.
type AssignedPort struct {
	ContainerPort int    // Port inside the sandbox
	HostPort      int    // Actual port assigned on the host
	HostIP        string // Host IP address (typically "0.0.0.0" or "127.0.0.1")
	Protocol      string // Protocol: "tcp" or "udp"
}

// SandboxStatus represents the current state of a sandbox.
type SandboxStatus string

const (
	StatusCreated SandboxStatus = "created" // Sandbox exists but not started
	StatusRunning SandboxStatus = "running" // Sandbox is running
	StatusStopped SandboxStatus = "stopped" // Sandbox has stopped
	StatusFailed  SandboxStatus = "failed"  // Sandbox failed to start or crashed
)

// CreateOptions configures sandbox creation.
type CreateOptions struct {
	Image   string            // Sandbox image (e.g., "ubuntu:22.04")
	Cmd     []string          // Command to run (empty = image default)
	WorkDir string            // Working directory inside sandbox
	Env     map[string]string // Environment variables
	Labels  map[string]string // Sandbox labels/tags for identification

	// Storage configures how workspace files are made available.
	// Interpretation is runtime-specific (Docker mounts, K8s PVCs, etc.)
	Storage StorageConfig

	// Resources defines resource limits for the sandbox.
	Resources ResourceConfig

	// Ports configures port mappings for the sandbox.
	// Maps sandbox ports to host ports.
	Ports []PortMapping
}

// PortMapping defines a port mapping from sandbox to host.
type PortMapping struct {
	ContainerPort int    // Port inside the sandbox
	HostPort      int    // Port on the host (0 = random available port)
	Protocol      string // Protocol: "tcp" or "udp" (default: "tcp")
}

// StorageConfig defines how workspace files are made available to the sandbox.
// The actual implementation varies by runtime:
// - Docker: bind mounts
// - Kubernetes: PersistentVolumeClaims
// - Cloudflare: R2/KV storage
type StorageConfig struct {
	WorkspacePath string // Host/source path to workspace
	MountPath     string // Path inside sandbox where workspace appears
	ReadOnly      bool   // Whether mount is read-only
}

// ResourceConfig defines resource limits for the sandbox.
type ResourceConfig struct {
	MemoryMB int           // Memory limit in MB (0 = no limit)
	CPUCores float64       // CPU cores (0 = no limit)
	DiskMB   int           // Disk space in MB (0 = no limit)
	Timeout  time.Duration // Max sandbox lifetime (0 = no limit)
}

// ExecOptions configures non-interactive command execution.
type ExecOptions struct {
	WorkDir string            // Working directory for command
	Env     map[string]string // Additional environment variables
	User    string            // User to run as (empty = default)
	Stdin   io.Reader         // Optional stdin input
}

// ExecResult contains the result of a non-interactive command execution.
type ExecResult struct {
	ExitCode int    // Exit code of the command
	Stdout   []byte // Standard output
	Stderr   []byte // Standard error
}

// AttachOptions configures interactive PTY session creation.
type AttachOptions struct {
	Cmd  []string          // Command to run (empty = default shell)
	Rows int               // Terminal rows
	Cols int               // Terminal columns
	Env  map[string]string // Additional environment variables
}

// PTY represents an interactive terminal session to a sandbox.
// It implements io.ReadWriteCloser for terminal I/O.
type PTY interface {
	// Read reads output from the PTY.
	// Implements io.Reader.
	Read(p []byte) (n int, err error)

	// Write sends input to the PTY.
	// Implements io.Writer.
	Write(p []byte) (n int, err error)

	// Resize changes the terminal dimensions.
	Resize(ctx context.Context, rows, cols int) error

	// Close terminates the PTY session.
	// Implements io.Closer.
	Close() error

	// Wait blocks until the PTY command exits and returns the exit code.
	// The context can be used to cancel the wait.
	Wait(ctx context.Context) (int, error)
}
