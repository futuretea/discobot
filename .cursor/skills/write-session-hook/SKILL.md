---
name: write-session-hook
description: Write a Discobot session hook for environment setup. Use when asked to create a hook, setup initialization script, or configure container startup tasks. Helps with dependency installation, build steps, and environment configuration.
metadata:
  argument-hint: "<hook-purpose>"
---

# Write Session Hook

Session hooks are executable scripts that run during container startup to prepare the development environment. They execute before the AI agent starts, making them ideal for dependency installation and environment setup.

## Quick Start

Create a file in `.discobot/hooks/` with:
1. Executable permissions (`chmod +x`)
2. A shebang line (`#!/bin/bash`)
3. YAML frontmatter with `type: session`

## Frontmatter Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | **Yes** | - | Must be `session` |
| `name` | string | No | filename | Display name in UI |
| `run_as` | `root` \| `user` | No | `user` | Execution user |
| `blocking` | boolean | No | `false` | Block startup until complete |

## Templates

### 1. Install Node Dependencies

```bash
#!/bin/bash
#---
# name: Install Node dependencies
# type: session
#---
cd /home/discobot/workspace
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1
```

### 2. Install System Packages (requires root)

```bash
#!/bin/bash
#---
# name: Install system packages
# type: session
# run_as: root
#---
apt-get update
apt-get install -y postgresql-client redis-tools
```

### 3. Parallel Go Module Downloads

```bash
#!/bin/bash
#---
# name: Download Go modules
# type: session
#---
cd /home/discobot/workspace

# Run downloads in parallel
cd server && go mod download 2>&1 &
cd proxy && go mod download 2>&1 &
cd agent && go mod download 2>&1 &

# Wait for all to complete
wait
```

### 4. Build Frontend

```bash
#!/bin/bash
#---
# name: Build frontend
# type: session
# blocking: true
#---
cd /home/discobot/workspace
pnpm install --frozen-lockfile 2>&1
pnpm run build 2>&1
```

### 5. Setup Environment File

```bash
#!/bin/bash
#---
# name: Setup environment
# type: session
#---
cd /home/discobot/workspace

if [ ! -f .env ]; then
    cp .env.example .env
    echo "Created .env from .env.example"
fi
```

### 6. Multiple Package Managers

```bash
#!/bin/bash
#---
# name: Install all dependencies
# type: session
#---
cd /home/discobot/workspace

# Node.js
pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1

# Go modules (parallel)
(cd server && go mod download 2>&1) &
(cd proxy && go mod download 2>&1) &
(cd agent && go mod download 2>&1) &
wait

# Python (if needed)
cd agent-api && pip install -r requirements.txt 2>&1
```

## Execution Order

Hooks run alphabetically by filename. Use numeric prefixes to control order:

```
.discobot/hooks/
├── 01-install-system-deps.sh   # Runs first
├── 02-install-node-deps.sh     # Runs second
└── 03-build-assets.sh          # Runs third
```

## Environment Variables

Available during execution:

| Variable | Value | Description |
|----------|-------|-------------|
| `DISCOBOT_SESSION_ID` | Session UUID | Current session identifier |
| `DISCOBOT_WORKSPACE` | `/home/discobot/workspace` | Project root path |
| `DISCOBOT_HOOK_TYPE` | `session` | Hook type identifier |
| `HOME` | `/home/discobot` | User home directory |
| `USER` | `discobot` | Username |

## Best Practices

### User Context

- Use `run_as: user` (default) for project commands (`pnpm`, `go`, `npm`)
- Use `run_as: root` only for system package installation (`apt-get`)

### Error Handling

- Always append `2>&1` to capture stderr in logs
- Use `||` for fallback commands
- Hooks can fail without blocking startup (unless `blocking: true`)

### Performance

- Keep hooks under 5 minutes (timeout limit)
- Run independent operations in parallel with `&` and `wait`
- Use `--frozen-lockfile` when possible for reproducible installs

### Idempotency

Hooks should be safe to run multiple times:

```bash
# Good: Check before creating
if [ ! -f .env ]; then
    cp .env.example .env
fi

# Good: Use lockfile for determinism
pnpm install --frozen-lockfile

# Avoid: Blind overwriting
cp .env.example .env  # May overwrite user changes
```

## Common Mistakes

❌ **Missing shebang**
```bash
#---
# type: session
#---
echo "Hello"  # Won't execute - no interpreter
```

✅ **Correct**
```bash
#!/bin/bash
#---
# type: session
#---
echo "Hello"
```

❌ **Not executable**
```bash
touch .discobot/hooks/my-hook.sh  # File exists but not executable
```

✅ **Correct**
```bash
touch .discobot/hooks/my-hook.sh
chmod +x .discobot/hooks/my-hook.sh
```

❌ **Wrong type**
```bash
#!/bin/bash
#---
# type: pre-commit  # Wrong - this won't run at session startup
#---
pnpm install
```

✅ **Correct**
```bash
#!/bin/bash
#---
# type: session  # Correct - runs at container startup
#---
pnpm install
```

## Debugging

View hook status and output in the Discobot UI:
- Hook status: Shows success/failure for each hook
- Output logs: Click a hook to see its full output
- Re-run: Manually trigger a hook to re-execute

Or check directly in the container:
```bash
cat ~/.discobot/{session-id}/hooks/status.json
ls ~/.discobot/{session-id}/hooks/output/
```
