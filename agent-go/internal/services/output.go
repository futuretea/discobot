package services

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// maxOutputSize is the maximum output file size before truncation (1MB).
const maxOutputSize = 1024 * 1024

// OutputEvent represents a single output event from a service.
type OutputEvent struct {
	Type      string `json:"type"`               // "stdout", "stderr", "exit", "error"
	Data      string `json:"data,omitempty"`     // stdout/stderr text
	ExitCode  *int   `json:"exitCode,omitempty"` // for "exit"
	Error     string `json:"error,omitempty"`    // for "error"
	Timestamp string `json:"timestamp"`          // ISO 8601
}

// outputDir returns the directory for service output files.
func outputDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "discobot", "services", "output")
}

// outputPath returns the path to a service's output file.
func outputPath(serviceID string) string {
	return filepath.Join(outputDir(), serviceID+".out")
}

// appendEvent appends an output event to the service's JSONL file.
func appendEvent(serviceID string, event OutputEvent) {
	dir := outputDir()
	_ = os.MkdirAll(dir, 0o755)

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	f, err := os.OpenFile(outputPath(serviceID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = f.Write(append(data, '\n'))

	// 1% chance to truncate
	if rand.Float64() < 0.01 {
		truncateIfNeeded(serviceID)
	}
}

// readEvents reads all output events from the service's JSONL file.
func readEvents(serviceID string) []OutputEvent {
	data, err := os.ReadFile(outputPath(serviceID))
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var events []OutputEvent
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event OutputEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events
}

// clearOutput truncates the service's output file.
func clearOutput(serviceID string) {
	path := outputPath(serviceID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	_ = f.Truncate(0)
	_ = f.Sync()
	f.Close()
}

// truncateIfNeeded truncates the output file if it exceeds maxOutputSize,
// keeping the bottom half of lines.
func truncateIfNeeded(serviceID string) {
	path := outputPath(serviceID)
	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxOutputSize {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	half := len(lines) / 2
	kept := strings.Join(lines[half:], "\n")
	_ = os.WriteFile(path, []byte(kept), 0o644)
}

// newStdoutEvent creates a stdout output event.
func newStdoutEvent(data string) OutputEvent {
	return OutputEvent{Type: "stdout", Data: data, Timestamp: time.Now().UTC().Format(time.RFC3339)}
}

// newStderrEvent creates a stderr output event.
func newStderrEvent(data string) OutputEvent {
	return OutputEvent{Type: "stderr", Data: data, Timestamp: time.Now().UTC().Format(time.RFC3339)}
}

// newExitEvent creates an exit output event.
func newExitEvent(exitCode *int) OutputEvent {
	return OutputEvent{Type: "exit", ExitCode: exitCode, Timestamp: time.Now().UTC().Format(time.RFC3339)}
}

// newErrorEvent creates an error output event.
func newErrorEvent(errMsg string) OutputEvent {
	return OutputEvent{Type: "error", Error: errMsg, Timestamp: time.Now().UTC().Format(time.RFC3339)}
}
