// Package conntrack provides a thread-safe tracker for active connections per session.
// It is used by the SSH server and service proxy middleware to register live connections,
// and by the idle monitor to avoid shutting down sandboxes that still have active connections.
package conntrack

import "sync"

// Tracker counts active connections per session ID.
type Tracker struct {
	mu    sync.Mutex
	conns map[string]int
}

// New returns a new Tracker.
func New() *Tracker {
	return &Tracker{conns: make(map[string]int)}
}

// Track registers an active connection for sessionID and returns a release function
// that must be called (typically via defer) when the connection ends.
func (t *Tracker) Track(sessionID string) func() {
	t.mu.Lock()
	t.conns[sessionID]++
	t.mu.Unlock()
	return func() {
		t.mu.Lock()
		t.conns[sessionID]--
		if t.conns[sessionID] <= 0 {
			delete(t.conns, sessionID)
		}
		t.mu.Unlock()
	}
}

// HasActiveConnections reports whether sessionID currently has any tracked connections.
func (t *Tracker) HasActiveConnections(sessionID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conns[sessionID] > 0
}

// ActiveCount returns the number of tracked connections for sessionID.
func (t *Tracker) ActiveCount(sessionID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conns[sessionID]
}
