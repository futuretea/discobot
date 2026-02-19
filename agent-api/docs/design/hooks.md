# Hooks Module

The hooks module enables workspace repositories to define automation scripts in `.discobot/hooks/`. Hooks run at specific lifecycle points: session startup, file changes, and pre-commit.

## Overview

Hooks are executable scripts with YAML front matter (same format as `.discobot/services/*`). A `type` field in the front matter determines when the hook runs.

Three hook types are supported:

1. **Session hooks** (`type: session`): Run once at container startup. Executed by the Go agent init process before the agent-api starts. Support running as root for system-level setup.

2. **File hooks** (`type: file`): Run at the end of each LLM turn when files matching a glob pattern have changed. On failure, the LLM is re-prompted with the hook output so it can fix the issue.

3. **Pre-commit hooks** (`type: pre-commit`): Installed as git pre-commit hooks. Run automatically when `git commit` is executed. Failures block the commit and are visible to the LLM via git's exit code.

## Hook File Format

### File Location

```
workspace/.discobot/hooks/
├── install-deps.sh      # Session hook
├── go-fmt.sh            # File hook
├── eslint-fix.sh        # File hook (silent)
└── typecheck.sh         # Pre-commit hook
```

### Front Matter

Same delimiter styles as services (`---`, `#---`, `//---`):

```bash
#!/bin/bash
#---
# name: Go format check
# type: file
# pattern: "*.go"
#---
gofmt -l $DISCOBOT_CHANGED_FILES
```

### Common Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | no | Display name (defaults to filename) |
| `type` | string | **yes** | `session`, `file`, or `pre-commit` |
| `description` | string | no | Human-readable description |

### Session Hook Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `run_as` | `root` \| `user` | `user` | Execute as root or as the discobot user |

### File Hook Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `pattern` | string | **required** | Glob pattern for file matching (e.g., `*.go`, `src/**/*.ts`) |
| `notify_llm` | boolean | `true` | Whether to re-prompt the LLM on hook failure |

### Pre-commit Hook Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `notify_llm` | boolean | `true` | Reserved for future use. Pre-commit failures are always visible via git exit code. |

## Examples

### Session Hook — Install system packages (as root)

```bash
#!/bin/bash
#---
# name: Install system deps
# type: session
# run_as: root
#---
apt-get update && apt-get install -y postgresql-client redis-tools
```

### Session Hook — Setup dev environment (as user)

```bash
#!/bin/bash
#---
# name: Setup environment
# type: session
#---
pnpm install
cp .env.example .env
```

### File Hook — Go formatting

```bash
#!/bin/bash
#---
# name: Go format check
# type: file
# pattern: "*.go"
#---
gofmt -l $DISCOBOT_CHANGED_FILES
```

### File Hook — Silent autofix (no LLM notification)

```bash
#!/bin/bash
#---
# name: ESLint autofix
# type: file
# pattern: "*.{ts,tsx}"
# notify_llm: false
#---
npx eslint --fix $DISCOBOT_CHANGED_FILES
```

### Pre-commit Hook

```bash
#!/bin/bash
#---
# name: Type check
# type: pre-commit
#---
pnpm typecheck
```

## Architecture

```
.discobot/hooks/*
       │
       ├──── Agent (Go init, PID 1) ────── Session hooks
       │                                    (runs before agent-api starts)
       │
       └──── Agent-API (TypeScript) ─────┬─ File hooks
                                         │  (end-of-turn evaluation)
                                         │
                                         └─ Pre-commit hooks
                                            (git hook installation)
```

## Session Hooks (Go Agent)

### Execution Point

Between workspace symlink creation (step 5) and agent-api startup (step 9) in `agent/cmd/agent/main.go`.

### Flow

1. Scan `/home/discobot/workspace/.discobot/hooks/` for executable files
2. Parse front matter, filter for `type: session`
3. Sort alphabetically for deterministic execution order
4. For each hook:
   - If `run_as: root` → execute as current user (root)
   - If `run_as: user` (default) → execute as discobot user via `syscall.Credential`
   - Set `cwd` to workspace directory
   - Capture stdout/stderr, log output
   - Timeout: 5 minutes per hook
   - **On failure: log error and continue** (don't block session startup)
5. Log total hooks executed and any failures

### Environment Variables

- `DISCOBOT_SESSION_ID` — Current session ID
- `DISCOBOT_WORKSPACE` — Workspace path
- `DISCOBOT_HOOK_TYPE` — `session`
- Standard user env (`HOME`, `USER`, proxy vars if enabled)

## File Hooks (Agent-API)

### Execution Timing

File hooks run **non-blocking after each completion finishes** (not inside the completion loop). This means:

- The LLM completion finishes cleanly and the SSE stream closes
- Hook evaluation starts in the background after a short grace period
- If a hook fails with `notify_llm`, a new completion is automatically triggered
- The new completion appears as a normal completion to the frontend (same SSE events)

This decouples the completion from hook evaluation, allowing the user to interact with the UI between hook evaluations.

### Change Detection

Uses a **marker file** at `~/.discobot/{sessionId}/hooks/.last-eval`:

- After each hook evaluation, touch the marker file
- On next evaluation, find workspace files with `mtime > marker mtime`
- First run (no marker): use `git diff --name-only HEAD` + `git ls-files --others --exclude-standard` to find all uncommitted/untracked changes

### Non-Blocking Hook Flow

Uses a **pending-hook tracking model** to ensure hooks aren't missed across turns.
When Hook-A fails (blocking Hook-B), Hook-B stays pending until Hook-A is resolved.

```
// completion.ts — orchestrates the non-blocking flow
runCompletion(userMessage):
  stream agent.prompt(userMessage) → SSE chunks to client
  finishCompletion()
  scheduleHookEvaluation()  // non-blocking

scheduleHookEvaluation():
  wait 200ms  // let SSE handler flush final events
  if aborted → return  // user started a new completion

  result = hookManager.evaluateFileHooks()

  if aborted → return
  if not result.shouldReprompt → return  // all hooks passed

  hookRetryCount++
  if hookRetryCount > MAX_HOOK_RETRIES → log warning, return

  tryStartCompletion(agent, hookFailureMessage, ...)
    → runs runCompletion(hookFailureMessage)
    → which calls scheduleHookEvaluation() again when done

// manager.ts — evaluateFileHooks (unchanged)
evaluateFileHooks():
  // Step 2: Find files changed since marker
  newFiles = findChangedFilesSinceMarker()

  // Step 3: Mark matching hooks as pending
  if newFiles.length > 0:
    matches = matchHooksToFiles(newFiles)
    if matches.length > 0:
      addPendingHooks(matches.map(m => m.hook.id))

  // Step 4: Always advance marker
  touchMarker()

  // Step 5: Run all pending hooks, stop on first failure
  pendingIds = getPendingHookIds()
  if no pendingIds → return noAction

  allDirtyFiles = getAllDirtyFiles()

  for hook in fileHooks (discovery order):
    if hook.id not in pendingIds → skip

    matchingFiles = allDirtyFiles.filter(picomatch(hook.pattern))
    if no matchingFiles:
      removePendingHook(hook.id)
      continue

    result = executeHook(hook, matchingFiles)
    persistStatus(hook, result)

    if result.success:
      removePendingHook(hook.id)
      continue

    if hook.notifyLlm → return shouldReprompt
    else → return evaluated (stop processing)

  return evaluated (all cleared)
```

**Example: Files A and B changed, Hook-A and Hook-B both match**

- Completion 1 finishes → scheduleHookEvaluation()
  - A,B changed → Hook-A,Hook-B marked pending. Marker touched.
  - Run Hook-A → fails. Hook-B stays pending.
  - tryStartCompletion(Hook-A failure message)
- Completion 2 (hook-triggered) → LLM fixes A → finishes → scheduleHookEvaluation()
  - A changed again → Hook-A already pending (no-op). Marker touched.
  - Run Hook-A → passes (removed). Run Hook-B → passes (removed). Done.

### Race Conditions

- **User sends message while hooks are evaluating**: `tryStartCompletion()` aborts hook evaluation via `hookAbortController`. User's completion runs. Pending hooks stay in `status.json` and re-evaluate after the user's completion.
- **User sends message while hook-triggered completion is running**: Returns 409 Conflict (same as any concurrent completion). User can cancel via POST /chat/cancel.

### Loop Guard

After **3 consecutive** hook-triggered completions, stop and log a warning. Pending hooks remain in `status.json` and will re-evaluate on the next user-initiated completion (which resets `hookRetryCount`).

### LLM Notification Format

Hook output is saved to `~/.discobot/{sessionId}/hooks/output/{hookId}.log`.

**Small output** (≤ 200 lines / 5KB) — included inline:

```
[Discobot Hook Failed] "Go format check" (pattern: *.go)

Files: internal/server/handler.go, internal/server/service.go
Exit code: 1

Output:
internal/server/handler.go: formatting differs from gofmt
internal/server/service.go: formatting differs from gofmt

Please fix the issues and ensure the hook passes. (Attempt 1/3)
```

**Large output** — reference the file:

```
[Discobot Hook Failed] "Go format check" (pattern: *.go)

Files: internal/server/handler.go, internal/server/service.go
Exit code: 1

Output is large (847 lines). Full output saved to:
  ~/.discobot/abc123/hooks/output/go-format-check.log

Please read the file to see the full output and address the issues. (Attempt 1/3)
```

### Environment Variables

- `DISCOBOT_CHANGED_FILES` — Space-separated list of changed file paths (relative to workspace)
- `DISCOBOT_HOOK_TYPE` — `file`
- `DISCOBOT_SESSION_ID` — Current session ID

## Pre-commit Hooks (Agent-API)

### Git Hook Installation

On agent-api startup, if pre-commit hooks are discovered:

1. Check if `.git/hooks/pre-commit` already exists
2. If it exists and wasn't created by discobot, preserve it as `.git/hooks/pre-commit.original`
3. Generate a `.git/hooks/pre-commit` that runs all `type: pre-commit` hooks

### Generated Script

```bash
#!/bin/bash
# Auto-generated by discobot hooks system
# Source: .discobot/hooks/
# DO NOT EDIT — regenerated when hooks change
# discobot:managed

set -e

# Run preserved original pre-commit hook
if [ -f .git/hooks/pre-commit.original ]; then
  .git/hooks/pre-commit.original
fi

# Run discobot pre-commit hooks
/home/discobot/workspace/.discobot/hooks/typecheck.sh
/home/discobot/workspace/.discobot/hooks/lint.sh
```

### LLM Integration

Pre-commit hooks integrate with the LLM naturally:

1. LLM runs `git commit` via the Bash tool
2. Git executes `.git/hooks/pre-commit`
3. If any hook fails, `git commit` exits non-zero
4. The LLM sees the failure and error output in the tool result
5. The LLM can fix the issue and retry

No special agent-api interception is needed.

## Status Persistence

### Storage Location

```
~/.discobot/{sessionId}/hooks/
  status.json              # Hook run status
  .last-eval               # Marker file for change detection
  output/
    {hookId}.log           # Latest output per hook (overwritten each run)
```

### Status Schema

```typescript
interface HookRunStatus {
  hookId: string;
  hookName: string;
  type: "session" | "file" | "pre-commit";
  lastRunAt: string;          // ISO timestamp
  lastResult: "success" | "failure";
  lastExitCode: number;
  outputPath: string;         // Path to output log file
  runCount: number;
  failCount: number;
  consecutiveFailures: number;
}

interface HookStatusFile {
  hooks: Record<string, HookRunStatus>;  // keyed by hookId
  pendingHooks: string[];                // hook IDs that need to run
  lastEvaluatedAt: string;               // ISO timestamp
}
```

### Write Strategy

Read-modify-write with atomic file writes (write to temp + rename). Updated after each hook execution.

## Module Structure

```
agent-api/src/hooks/
├── parser.ts          # Hook discovery and front matter parsing
├── executor.ts        # Script execution with timeout and output capture
├── status.ts          # Persistent status store (read/write status.json)
├── manager.ts         # File hook orchestration (change detection, matching, loop)
└── pre-commit.ts      # Git pre-commit hook installation

agent/cmd/agent/
└── hooks.go           # Session hook discovery, parsing, execution (Go)
```

## Testing

- **Parser tests**: Hook front matter parsing, type-specific field extraction
- **Executor tests**: Script execution, timeout, output capture, exit codes
- **Status tests**: Read/write/update status.json, atomic writes
- **Manager tests**: Change detection, pattern matching, loop guard, notification formatting
- **Pre-commit tests**: Git hook generation, chaining with existing hooks
- **Go session hook tests**: Discovery, execution order, run_as, timeout
