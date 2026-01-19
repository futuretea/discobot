# Agent Architecture

This document describes the architecture of the `obot-agent` init process.

## Overview

The `obot-agent` binary serves as the container's PID 1 process, providing:

1. Home directory initialization (copy from template)
2. Workspace initialization (git clone)
3. AgentFS setup (copy-on-write filesystem)
4. Process reaping for zombie collection
5. Privilege separation (root → octobot user)
6. Signal handling and forwarding
7. Graceful shutdown coordination

## Design Goals

### Minimal and Secure

- Statically compiled Go binary with no runtime dependencies
- Drops root privileges immediately after setup completes
- Uses `pdeathsig` to ensure child processes terminate with parent

### Container-Native

- Designed specifically for Docker/OCI container environments
- Handles PID 1 responsibilities that the Linux kernel delegates to init
- Works with both Docker runtime and VZ (Virtualization.framework) VMs

### Copy-on-Write Isolation

- Uses AgentFS for efficient storage with snapshot/restore capability
- Base layer is read-only (home directory + cloned repository)
- Agent sees a writable overlay that captures all changes
- Changes are stored efficiently in SQLite database

## Startup Sequence

```
┌─────────────────────────────────────────────────────────────┐
│                    Container Start                          │
│                    (running as root)                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 1: Base Home Setup                                    │
│  ───────────────────────                                    │
│  • Check if /.data/octobot exists                           │
│  • If not, copy /home/octobot to /.data/octobot             │
│  • Preserve all permissions and ownership                   │
│  • Set ownership to octobot user                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 2: Workspace Clone                                    │
│  ─────────────────────────                                  │
│  • Check if /.data/octobot/workspace exists                 │
│  • If not, clone WORKSPACE_PATH to staging directory        │
│  • Checkout WORKSPACE_COMMIT if specified                   │
│  • Change ownership to octobot user                         │
│  • Atomically rename staging → workspace                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 3: AgentFS Directory                                  │
│  ─────────────────────────                                  │
│  • Create /.data/.agentfs directory                         │
│  • Set ownership to octobot user                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 3b: Session Directory                                 │
│  ──────────────────────────                                 │
│  • Create /.data/session directory                          │
│  • Set ownership to octobot user                            │
│  • Used by agent-api for session/message persistence        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 4: AgentFS Init                                       │
│  ───────────────────                                        │
│  • Check if /.data/.agentfs/{SESSION_ID}.db exists          │
│  • If not, run: agentfs init --base /.data/octobot {id}     │
│  • Creates SQLite database with base layer reference        │
│  • Runs as octobot user                                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 5: AgentFS Mount                                      │
│  ──────────────────                                         │
│  • Run as octobot user:                                     │
│    agentfs mount -a --allow-root {id} /home/octobot         │
│  • -a: auto-unmount on exit                                 │
│  • --allow-root: allow root access to FUSE mount            │
│  • Mounts COW filesystem directly over /home/octobot        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 6: Create Workspace Symlink                           │
│  ───────────────────────────────                            │
│  • Create /workspace -> /home/octobot/workspace symlink     │
│  • Provides convenient access to project directory          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 7: Run Agent API                                      │
│  ─────────────────────                                      │
│  • Fork child process                                       │
│  • Switch to octobot user (setuid/setgid)                   │
│  • Set HOME, USER, LOGNAME environment                      │
│  • Set working directory to /home/octobot/workspace         │
│  • Configure pdeathsig for cleanup                          │
│  • Enter event loop for signal handling                     │
└─────────────────────────────────────────────────────────────┘
```

## Component Design

```
┌─────────────────────────────────────────────────────────────┐
│                     obot-agent (PID 1)                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   Home       │  │   Workspace  │  │      AgentFS     │   │
│  │   Manager    │  │   Manager    │  │      Manager     │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │   Signal     │  │   Process    │  │      Child       │   │
│  │   Handler    │  │   Reaper     │  │      Manager     │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Home Manager

Handles initial home directory setup:

- Copies /home/octobot template to /.data/octobot
- Preserves file permissions and ownership
- Recursive copy with symlink support
- Only runs on first container start

### Workspace Manager

Handles git operations for workspace initialization:

- Clones to staging directory first for atomicity
- Supports specific commit checkout
- Skips clone if workspace already exists
- Creates empty workspace if no WORKSPACE_PATH

### AgentFS Manager

Integrates with the AgentFS copy-on-write filesystem:

- Initializes database with base layer reference
- Mounts FUSE filesystem directly over /home/octobot
- Uses `-a` flag for auto-unmount on exit
- Uses `--allow-root` for root access (docker exec)
- Provides efficient storage for session changes

### Signal Handler

Handles incoming signals and forwards them appropriately:

| Signal | Action |
|--------|--------|
| SIGTERM | Forward to child, start shutdown timer |
| SIGINT | Forward to child, start shutdown timer |
| SIGQUIT | Forward to child, start shutdown timer |
| SIGHUP | Forward to child (config reload) |
| SIGCHLD | Trigger process reaping |

### Process Reaper

Collects zombie processes using `wait4()` with `WNOHANG`. This is essential for PID 1 because:

- Orphaned processes are re-parented to PID 1
- Only PID 1 can reap these orphans
- Without reaping, zombies accumulate and consume process table entries

### Child Manager

Responsible for:

1. **User Lookup**: Resolves username to UID/GID
2. **Environment Setup**: Sets HOME, USER, LOGNAME
3. **Process Creation**: Forks with credential switching
4. **Pdeathsig Setup**: Configures child to receive SIGTERM on parent death
5. **Exit Handling**: Captures and propagates child exit code

## Filesystem Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     What Agent Sees                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  /home/octobot  ─────► AgentFS FUSE mount (COW layer)       │
│  /workspace     ─────► symlink to /home/octobot/workspace   │
│                                                             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                  Actual Filesystem Layout                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  /.data/                                                    │
│  ├── octobot/                   (base layer, read-only)     │
│  │   ├── .bashrc                (shell config)              │
│  │   ├── .profile               (user profile)              │
│  │   └── workspace/             (cloned repository)         │
│  ├── .agentfs/                                              │
│  │   └── {SESSION_ID}.db        (SQLite with changes)       │
│  └── session/                   (agent-api persistence)     │
│      ├── agent-session.json     (session metadata)          │
│      └── agent-messages.json    (message history)           │
│                                                             │
│  /home/octobot                  (AgentFS FUSE mount)        │
│  └── workspace/                 (COW of /.data/octobot/ws)  │
│                                                             │
│  /workspace -> /home/octobot/workspace (symlink)            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Security Considerations

### Privilege Separation

The init process runs as root to:
- Copy home directory template
- Clone git repositories
- Create directories with correct ownership
- Perform mount operations
- Initialize AgentFS database

The child process runs as the `octobot` user with:
- No root access
- Standard user filesystem permissions
- Working directory set to /home/octobot/workspace

### FUSE Mount Access

The `--allow-root` flag on the AgentFS mount allows:
- Root to access the FUSE filesystem (needed for docker exec)
- The octobot user to read/write normally
- Requires `user_allow_other` in /etc/fuse.conf

### Pdeathsig

The `PR_SET_PDEATHSIG` option ensures the child receives SIGTERM if the parent dies unexpectedly. This prevents orphaned agent processes from continuing to run.

## Design Documents

- [Init Process Design](./design/init.md) - Detailed init process implementation
