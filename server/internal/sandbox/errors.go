package sandbox

import "errors"

// Sentinel errors for sandbox operations.
var (
	// ErrNotFound indicates the sandbox does not exist.
	ErrNotFound = errors.New("sandbox not found")

	// ErrAlreadyExists indicates a sandbox already exists for the session.
	ErrAlreadyExists = errors.New("sandbox already exists for session")

	// ErrNotRunning indicates the sandbox is not running when it should be.
	ErrNotRunning = errors.New("sandbox not running")

	// ErrAlreadyRunning indicates the sandbox is already running.
	ErrAlreadyRunning = errors.New("sandbox already running")

	// ErrStartFailed indicates the sandbox failed to start.
	ErrStartFailed = errors.New("sandbox failed to start")

	// ErrExecFailed indicates command execution failed.
	ErrExecFailed = errors.New("command execution failed")

	// ErrAttachFailed indicates failed to attach to sandbox PTY.
	ErrAttachFailed = errors.New("failed to attach to sandbox")

	// ErrTimeout indicates the operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrInvalidImage indicates the sandbox image is invalid or not found.
	ErrInvalidImage = errors.New("invalid sandbox image")

	// ErrResourceLimit indicates a resource limit was exceeded.
	ErrResourceLimit = errors.New("resource limit exceeded")
)
