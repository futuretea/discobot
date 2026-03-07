#!/bin/bash

# Refresh models.dev data - downloads the latest API data for provider information
# Usage: ./scripts/refresh-models-data.sh
# Or via npm: npm run refresh-models
#
# Note: Provider logos are loaded directly from https://models.dev/logos/ CDN

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
SERVER_API_OUTPUT="$PROJECT_DIR/server/static/models-dev-api.json"
AGENT_API_OUTPUT="$PROJECT_DIR/agent-go/providers/modelsdev/models-dev-api.json"
TMP_OUTPUT="$(mktemp)"
trap 'rm -f "$TMP_OUTPUT"' EXIT

echo "Refreshing models.dev data..."

# Download api.json once, then copy to both server and agent-go locations
echo "Downloading models.dev API data..."
curl -s "https://models.dev/api.json" | jq --sort-keys '.' > "$TMP_OUTPUT"
cp "$TMP_OUTPUT" "$SERVER_API_OUTPUT"
cp "$TMP_OUTPUT" "$AGENT_API_OUTPUT"
echo "  Saved models-dev-api.json ($(wc -c < "$TMP_OUTPUT" | tr -d ' ') bytes)"

# Show summary
echo ""
echo "Summary:"
echo "  API data (server): $SERVER_API_OUTPUT"
echo "  API data (agent-go): $AGENT_API_OUTPUT"
echo "  Providers: $(jq 'keys | length' "$TMP_OUTPUT")"
