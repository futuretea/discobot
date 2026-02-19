#!/bin/bash
#---
# name: Go lint
# type: file
# pattern: "**/*.go"
#---
# Auto-fix formatting (gofmt + goimports) and lint Go files.
pnpm run lint:go:fix
