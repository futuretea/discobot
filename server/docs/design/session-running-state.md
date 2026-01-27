# Session Running State

This document describes the "running" state for sessions, which indicates when a chat completion is actively in progress.

## Overview

Sessions now transition between `ready` and `running` states to indicate whether a chat is actively being processed. This provides:
- Real-time UI feedback showing which sessions have active chats
- Reconciliation on server startup to detect stale "running" states
- Automatic cleanup when chats complete

## State Lifecycle

```
ready ⇄ running
  ↓
stopped/error
```

### State Transitions

| From | To | Trigger | Location |
|------|-----|---------|----------|
| `ready` | `running` | Chat request starts | `chat.go:SendToSandbox()` |
| `running` | `ready` | Chat request completes | `chat.go:Chat()` defer |
| `running` | `ready` | Server startup reconciliation | `sandbox.go:ReconcileSessionStates()` |

### State Meanings

- **`ready`**: Session is ready to accept chat requests
- **`running`**: Session has an active chat completion in progress
- **`stopped`**: Sandbox is stopped, will restart on demand
- **`error`**: Setup or operation failed

## Implementation

### Backend (Go Server)

#### Model Constants

```go
// server/internal/model/model.go
const (
    SessionStatusReady   = "ready"
    SessionStatusRunning = "running"
    // ... other statuses
)
```

#### Chat Flow

1. **Start of Chat** (`server/internal/service/chat.go`):
```go
func (c *ChatService) SendToSandbox(...) {
    // Set status to running
    c.sessionService.UpdateStatus(ctx, sessionID, model.SessionStatusRunning, nil)
    c.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, model.SessionStatusRunning, "")

    // Send messages to sandbox...
}
```

2. **End of Chat** (`server/internal/handler/chat.go`):
```go
defer func() {
    // Reset status to ready
    h.sessionService.UpdateStatus(ctx, sessionID, model.SessionStatusReady, nil)
    h.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, model.SessionStatusReady, "")
}()
```

#### Startup Reconciliation

On server startup, `ReconcileSessionStates()` checks all sessions marked as "running":

```go
// server/internal/service/sandbox.go
func (s *SandboxService) ReconcileSessionStates(ctx context.Context) error {
    // Query sessions in "running" state
    // For each session:
    //   1. Check if sandbox is actually running
    //   2. Query agent API /chat/status endpoint
    //   3. If not actually running, reset to "ready"
}
```

**Key Logic**:
- Queries agent API at `localhost:<port>/chat/status`
- If agent API returns `isRunning: false`, resets session to `ready`
- If agent API is unreachable, assumes not running and resets to `ready`
- This handles server crashes during active chats

### Agent API

The agent API tracks completion state in-memory:

```typescript
// agent-api/src/store/session.ts
interface CompletionState {
    isRunning: boolean
    completionId: string | null
    startedAt: string | null
    error: string | null
}
```

**Endpoints**:
- `GET /chat/status`: Returns current completion state
- Response: `{ isRunning: boolean, completionId: string | null, ... }`

**State Management**:
- Set to running when `POST /chat` starts completion
- Set to not running when completion finishes
- Purely in-memory (not persisted to disk)

### Frontend (TypeScript/React)

#### Constants

```typescript
// lib/api-constants.ts
export const SessionStatus = {
    READY: "ready",
    RUNNING: "running",
    // ... other statuses
} as const;
```

#### UI Indicators

The `getSessionStatusIndicator()` function in `lib/session-utils.tsx` provides icons:

| State | Icon | Color | Animation |
|-------|------|-------|-----------|
| `ready` | Circle (filled) | Green | Static |
| `running` | Loader2 | Blue | Spinning |

**Size Variants**:
- `"default"`: 3-3.5px (for dropdowns)
- `"small"`: 2.5px (for sidebar tree)

**Usage**:
```tsx
// Sidebar
getSessionStatusIndicator(session, "small")

// Dropdown
getSessionStatusIndicator(session)  // default
```

## Error Handling

### Scenarios

1. **Server Crashes During Chat**:
   - Status remains "running" in database
   - On restart, reconciliation detects stale state
   - Queries agent API, finds not running
   - Resets to "ready"

2. **Agent API Unavailable**:
   - Reconciliation assumes not running
   - Resets to "ready"
   - Safe default: prefer marking as ready over stuck "running"

3. **Network Issues**:
   - Retry logic with exponential backoff (up to 15 attempts)
   - Falls back to marking as "ready" on persistent failure

### Reconciliation Safety

**Idempotent**: Can run multiple times safely
**Conservative**: Defaults to "ready" when uncertain
**Non-blocking**: Doesn't prevent server startup

## Testing

### Integration Tests

```go
// server/internal/integration/sandbox_reconcile_test.go
TestReconcileSessionStates_ResetsRunningSessionWithNoActiveChat
```

**Test Scenario**:
1. Create session with status "running"
2. Create sandbox without starting a chat
3. Run reconciliation
4. Verify status reset to "ready"

### Unit Tests

```typescript
// lib/session-utils.test.tsx
- Icon rendering for running state (both sizes)
- Status precedence (commit status over session status)
- All state combinations
```

## SSE Events

Status changes emit SSE events for real-time UI updates:

```json
{
  "type": "session_updated",
  "projectId": "local",
  "sessionId": "abc-123",
  "status": "running"
}
```

Clients subscribe to `/api/projects/{id}/events` to receive updates.

## Performance Considerations

1. **Status Updates**: Minimal overhead (single DB update)
2. **Reconciliation**: Only runs on server startup
3. **Agent API Query**: Fast in-memory lookup (no I/O)
4. **SSE Events**: Published asynchronously (non-blocking)

## Migration Notes

**Backward Compatibility**:
- Existing sessions remain in "ready" state
- No database migration required (just a new enum value)
- Frontend handles unknown states gracefully (shows gray circle)

## Future Enhancements

Potential improvements:
1. Track completion progress percentage
2. Add "paused" state for interrupted chats
3. Store completion metadata (model, tokens, duration)
4. Support concurrent chat requests per session
