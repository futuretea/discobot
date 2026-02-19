#!/bin/bash
#---
# name: TypeScript typecheck
# type: file
# pattern: "**/*.{ts,tsx}"
#---
# Type checking is global — a change in one file can cause errors elsewhere.
pnpm typecheck
