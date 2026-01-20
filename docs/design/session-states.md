# Session States Design

This document describes the session lifecycle states and commit states, which are tracked independently.

## Overview

Sessions have two independent state dimensions:

1. **Session Status** (`status`): Tracks the lifecycle of the session (initialization, running, stopped, etc.)
2. **Commit Status** (`commitStatus`): Tracks commit operations (orthogonal to session status)

This separation allows a session to be `ready` and `committing` at the same time, which correctly models that the sandbox continues running while a commit is in progress.

## Session Status (Lifecycle)

### State Diagram

```
                                    ┌──────────────┐
                                    │ initializing │
                                    └──────┬───────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
                    ▼                      ▼                      ▼
            ┌───────────┐          ┌──────────────┐       ┌───────────────────┐
            │  cloning  │          │ pulling_image│       │ creating_sandbox  │
            └─────┬─────┘          └──────┬───────┘       └─────────┬─────────┘
                  │                       │                         │
                  └───────────────────────┼─────────────────────────┘
                                          │
                                          ▼
                                    ┌───────────┐
                           ┌────────│   ready   │────────┐
                           │        └─────┬─────┘        │
                           │              │              │
                           ▼              │              ▼
                     ┌──────────┐         │        ┌──────────┐
                     │ stopped  │◄────────┘        │  error   │
                     └────┬─────┘                  └──────────┘
                          │
                          ▼
                   ┌────────────┐
                   │  removing  │
                   └──────┬─────┘
                          │
                          ▼
                    ┌──────────┐
                    │ removed  │
                    └──────────┘
```

### Status Values

| Status | Description |
|--------|-------------|
| `initializing` | Session just created, starting setup process |
| `reinitializing` | Recreating sandbox after it was deleted (e.g., Docker container removed externally) |
| `cloning` | Cloning git repository for the workspace |
| `pulling_image` | Pulling the sandbox Docker image |
| `creating_sandbox` | Creating the sandbox container environment |
| `ready` | Session is ready for use. Sandbox is running and accepting commands. |
| `stopped` | Sandbox is stopped. Will be restarted on demand when user sends a message. |
| `error` | Something failed during setup. Check `errorMessage` for details. |
| `removing` | Session is being deleted asynchronously |
| `removed` | Session has been deleted. Client should remove from UI. |

## Commit Status (Orthogonal)

### State Diagram

```
    ┌─────────┐     commit()     ┌──────────┐     job starts    ┌────────────┐
    │  none   │ ───────────────► │ pending  │ ────────────────► │ committing │
    └─────────┘                  └──────────┘                   └──────┬─────┘
         ▲                                                             │
         │                                                   ┌─────────┴─────────┐
         │                                                   │                   │
         │                                           success │           failure │
         │                                                   ▼                   ▼
         │                                           ┌────────────┐       ┌──────────┐
         └─────────────────────────────────────────  │ completed  │       │  failed  │
              (can commit again after completed)     └────────────┘       └──────────┘
```

### Status Values

| Status | Description |
|--------|-------------|
| `""` (empty) | No commit in progress (default state) |
| `pending` | Commit requested, waiting for job to start |
| `committing` | Commit job is actively running |
| `completed` | Commit completed successfully |
| `failed` | Commit failed |

## Combined State Display

The UI displays a consolidated view of both states:

1. **If `commitStatus` is `pending` or `committing`**: Show commit progress indicator (takes priority)
2. **Otherwise**: Show the session `status` indicator

### UI Indicators

| State | Sidebar Indicator | Chat Header |
|-------|-------------------|-------------|
| Lifecycle states (initializing, cloning, etc.) | Yellow spinner | Status banner |
| `ready` | Green dot | No banner |
| `stopped` | Pause icon | Yellow banner |
| `error` | Red dot | Red banner with error message |
| `removing` | Red spinner | Red banner |
| `pending` / `committing` | Blue spinner | Blue banner |
| `completed` | Green dot (returns to `ready` display) | Brief success banner |
| `failed` | Red dot | Error banner |

## Chat Behavior

| Session Status | Commit Status | Chat Allowed |
|---------------|---------------|--------------|
| Any | `pending` | **No** - Input disabled with "Chat disabled during commit..." |
| Any | `committing` | **No** - Input disabled with "Chat disabled during commit..." |
| `ready` | `""` / `completed` | Yes |
| `stopped` | `""` / `completed` | Yes (restarts sandbox) |
| `error` | Any | No |
| Initialization states | Any | Yes (queued until ready) |
| `removing` / `removed` | Any | No |

## Server Restart Handling

The commit job is designed to handle server restarts:

1. **Job persists in database**: The `session_commit` job is stored with `pending` status
2. **Session state persists**: `commitStatus` is stored in the session record
3. **On restart**:
   - Job dispatcher picks up pending jobs
   - `PerformCommit()` checks `commitStatus`:
     - If `pending`: Continues normally (transitions to `committing`)
     - If `committing`: Continues from where it left off
     - If neither: Job exits gracefully (state was manually changed)

## Implementation Details

### Backend Components

- **Model**: `server/internal/model/model.go`
  - `Session.Status` - Lifecycle status
  - `Session.CommitStatus` - Commit status (new field)
  - Status and CommitStatus constants

- **Service**: `server/internal/service/session.go`
  - `CommitSession()` - Initiates commit (sets `commitStatus = "pending"`)
  - `PerformCommit()` - Executes commit job (transitions through commit states)
  - `updateCommitStatusWithEvent()` - Updates status and emits SSE

- **Job**: `server/internal/jobs/session_commit.go`
  - `SessionCommitExecutor` - Handles the commit job

- **Handler**: `server/internal/handler/sessions.go`
  - `POST /sessions/{id}/commit` - Initiates commit

- **Chat Handler**: `server/internal/handler/chat.go`
  - Blocks chat when `commitStatus` is `pending` or `committing`

### Frontend Components

- **Types**: `lib/api-types.ts`
  - `SessionStatus` - Lifecycle status type
  - `CommitStatus` - Commit status type
  - `Session.commitStatus` - New field

- **Chat Panel**: `components/ide/chat-panel.tsx`
  - `getStatusDisplay()` - Session lifecycle status
  - `getCommitStatusDisplay()` - Commit status
  - Input locking based on `commitStatus`

- **Sidebar**: `components/ide/sidebar-tree.tsx`
  - `getSessionStatusIndicator()` - Combined status indicator

- **Bottom Panel**: `components/ide/layout/bottom-panel.tsx`
  - Commit button with loading state based on `commitStatus`

### Database Schema

```sql
-- Session table
ALTER TABLE sessions ADD COLUMN commit_status TEXT DEFAULT '';
```

### API Endpoints

**Initiate Commit**
```
POST /api/projects/{projectId}/sessions/{sessionId}/commit
```

**Response**:
- `200 OK`: `{ "success": true }` - Commit initiated
- `404 Not Found`: Session not found
- `409 Conflict`: Commit already in progress

**Session Response** (includes new field)
```json
{
  "id": "abc123",
  "status": "ready",
  "commitStatus": "committing",
  ...
}
```

### SSE Events

Session updates are broadcast via SSE when commit status changes:

```json
{
  "type": "session_updated",
  "data": {
    "sessionId": "abc123",
    "status": ""
  }
}
```

The client should re-fetch the session to get the updated `commitStatus`.

## Future Considerations

1. **Commit Parameters**: The commit job currently sleeps for 10 seconds as a placeholder. Future implementation will:
   - Generate commit message from AI
   - Stage changed files
   - Create git commit
   - Optionally push to remote

2. **Commit Cancellation**: Allow cancelling a commit when `commitStatus` is `pending`.

3. **Commit History**: Track commit history per session.

4. **Branch Management**: Handle branch creation/switching as part of commit flow.

5. **Error Recovery**: Allow retrying failed commits without resetting the session.
