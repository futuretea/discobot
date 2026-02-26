#!/bin/bash
#---
# name: SQLite GUI
# description: sqlite-web browser for the Discobot database
# http: 8080
#---

set +x

DB="${HOME}/.local/share/discobot/discobot.db"

if [ ! -f "$DB" ]; then
    echo "Database not found at: $DB"
    echo "Start the API service first to create the database."
    exit 1
fi

echo "Opening SQLite GUI at http://localhost:8080"
echo "Database: $DB"

exec uvx --native-tls datasette serve "$DB" --port 8080 --host 0.0.0.0
