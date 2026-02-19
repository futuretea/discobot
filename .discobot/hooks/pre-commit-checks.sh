#!/bin/bash
#---
# name: Pre-commit quality checks
# type: pre-commit
#---
# Run the full CI pipeline: check:fix → test:unit → build.
pnpm run ci
