#!/bin/bash
#---
# name: Biome format & lint
# type: file
# pattern: "**/*.{ts,tsx,js,jsx}"
#---
# Auto-fix formatting and lint issues, then verify everything passes.
# Biome exits non-zero if unfixable issues remain.
pnpm biome check --write $DISCOBOT_CHANGED_FILES
