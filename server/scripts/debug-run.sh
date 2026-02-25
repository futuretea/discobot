#!/bin/bash
# Wrapper script for air: starts the server binary and generates .zed/debug.json with the PID
set -e

BINARY="./build/discobot"
DEBUG_JSON="../.zed/debug.json"

"$BINARY" "$@" &
PID=$!

mkdir -p "$(dirname "$DEBUG_JSON")"
cat > "$DEBUG_JSON" <<EOF
[
    {
        "adapter": "Delve",
        "label": "Attach to Discobot Server (Delve)",
        "request": "attach",
        "mode": "local",
        "processId": $PID,
        "cwd": "\${ZED_WORKTREE_ROOT}/server",
        "stopOnEntry": false
    }
]
EOF

wait $PID
