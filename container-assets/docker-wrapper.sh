#!/bin/bash
# Docker wrapper that ensures build commands load images into the local daemon.
#
# When using a remote buildx builder, "docker build" does NOT automatically
# load the built image into the local Docker image store. This wrapper detects
# build commands and injects "--output type=docker" when no output flag is
# specified, so images are always available locally after building.
#
# Install by placing earlier in PATH than the real docker binary:
#   cp docker-wrapper.sh /usr/local/bin/docker

set -euo pipefail

REAL_DOCKER=/usr/bin/docker

# Check if this is a build command that needs --output injection.
# Returns 0 (true) if we should add --output type=docker.
needs_output_flag() {
    local found_build=false
    local has_output=false

    for arg in "$@"; do
        case "$arg" in
            build)
                found_build=true
                ;;
            --output|--output=*|--load|--push|-o)
                has_output=true
                ;;
        esac
    done

    $found_build && ! $has_output
}

if needs_output_flag "$@"; then
    exec "$REAL_DOCKER" "$@" --output type=docker
else
    exec "$REAL_DOCKER" "$@"
fi
