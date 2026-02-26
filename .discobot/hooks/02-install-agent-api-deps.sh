#!/bin/bash
#---
# name: Update agent-api bun lockfile
# type: file
# pattern: "agent-api/package.json"
#---
# Update bun.lock without disturbing pnpm-managed node_modules.
# bun has no --lockfile-only mode, so we stash and restore node_modules.
cd agent-api || exit

# Stash pnpm node_modules
if [ -d node_modules ]; then
	mv node_modules node_modules._pnpm_stash
fi

# Restore pnpm node_modules on exit (success or failure)
restore() {
	rm -rf node_modules
	if [ -d node_modules._pnpm_stash ]; then
		mv node_modules._pnpm_stash node_modules
	fi
}
trap restore EXIT

bun install
