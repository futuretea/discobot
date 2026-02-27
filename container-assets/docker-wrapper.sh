#!/bin/bash
# Docker wrapper that ensures build commands:
# 1. Load the built image into the local daemon (--load).
# 2. Import and export a local BuildKit cache stored on the project cache volume.
#
# The cache lives at $HOME/.cache/discobot/buildkit, writable by the
# container user without any special permissions.
#
# Build commands ("docker build" or "docker buildx build") are transparently
# rewritten to "docker buildx build" so that --cache-from/--cache-to with
# type=local work correctly (they require the BuildKit frontend).
#
# Install by placing earlier in PATH than the real docker binary:
#   cp docker-wrapper.sh /usr/local/bin/docker

set -euo pipefail

REAL_DOCKER=/usr/bin/docker
CACHE_DIR="$HOME/.cache/discobot/buildkit"

# Returns 0 if this is a build subcommand (handles both
# "docker build ..." and "docker buildx build ...").
is_build_command() {
    [ "${1:-}" = "build" ] || { [ "${1:-}" = "buildx" ] && [ "${2:-}" = "build" ]; }
}

# Returns 0 if an output/load/push flag is already present.
has_output_flag() {
    for arg in "$@"; do
        case "$arg" in
            --load|--push|--output|--output=*|-o) return 0 ;;
        esac
    done
    return 1
}

# Returns 0 if cache flags are already present.
has_cache_flag() {
    for arg in "$@"; do
        case "$arg" in
            --cache-from|--cache-from=*|--cache-to|--cache-to=*) return 0 ;;
        esac
    done
    return 1
}

if is_build_command "$@"; then
    extra=()

    # Ensure the built image is loaded into the local daemon.
    if ! has_output_flag "$@"; then
        extra+=(--load)
    fi

    # Inject local cache import/export. mode=max exports all intermediate
    # layers for maximum cache reuse on subsequent builds.
    if ! has_cache_flag "$@"; then
        mkdir -p "$CACHE_DIR"
        extra+=(
            --cache-from "type=local,src=$CACHE_DIR"
            --cache-to "type=local,dest=$CACHE_DIR,mode=max"
        )
    fi

    if [ "${1:-}" = "build" ]; then
        # Rewrite "docker build ..." as "docker buildx build ..."
        # (type=local cache requires the BuildKit frontend, only available via buildx)
        shift
        exec "$REAL_DOCKER" buildx build "$@" "${extra[@]}"
    else
        exec "$REAL_DOCKER" "$@" "${extra[@]}"
    fi
else
    exec "$REAL_DOCKER" "$@"
fi
