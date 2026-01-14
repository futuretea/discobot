#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
SERVER_DIR="$PROJECT_ROOT/server"
BINARIES_DIR="$PROJECT_ROOT/src-tauri/binaries"

# Create binaries directory if it doesn't exist
mkdir -p "$BINARIES_DIR"

# Get the current platform's target triple
get_target_triple() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    case "$os" in
        linux)
            case "$arch" in
                x86_64) echo "x86_64-unknown-linux-gnu" ;;
                aarch64) echo "aarch64-unknown-linux-gnu" ;;
                *) echo "unknown" ;;
            esac
            ;;
        darwin)
            case "$arch" in
                x86_64) echo "x86_64-apple-darwin" ;;
                arm64) echo "aarch64-apple-darwin" ;;
                *) echo "unknown" ;;
            esac
            ;;
        mingw*|msys*|cygwin*)
            case "$arch" in
                x86_64) echo "x86_64-pc-windows-msvc" ;;
                *) echo "unknown" ;;
            esac
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

TARGET_TRIPLE=$(get_target_triple)

if [ "$TARGET_TRIPLE" = "unknown" ]; then
    echo "Error: Unsupported platform"
    exit 1
fi

echo "Building octobot-server for $TARGET_TRIPLE..."

cd "$SERVER_DIR"

# Build the Go binary
OUTPUT_NAME="octobot-server-$TARGET_TRIPLE"

# Add .exe extension on Windows
if [[ "$TARGET_TRIPLE" == *"windows"* ]]; then
    OUTPUT_NAME="$OUTPUT_NAME.exe"
fi

go build -o "$BINARIES_DIR/$OUTPUT_NAME" ./cmd/server

echo "Built: $BINARIES_DIR/$OUTPUT_NAME"
