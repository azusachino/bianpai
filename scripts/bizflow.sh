#!/bin/bash
set -euo pipefail

# Resolve project paths
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$DIR/.." && pwd)"

echo "=== Building bianpai ==="
go build -o "$PROJECT_DIR/bianpai-biz" "$PROJECT_DIR/main.go"
BIN="$PROJECT_DIR/bianpai-biz"

COMPOSE_FILE="$PROJECT_DIR/usecases/multi-host/compose.yaml"
SHARED_DIR="$PROJECT_DIR/usecases/multi-host/shared-data"

# Setup shared directory and clean stale state
mkdir -p "$SHARED_DIR"
"$BIN" --backend container -f "$COMPOSE_FILE" down -v >/dev/null 2>&1 || true

echo ""
echo "=== [1/8] Checking version ==="
"$BIN" --backend container version

echo ""
echo "=== [2/8] Pulling images ==="
"$BIN" --backend container -f "$COMPOSE_FILE" pull

echo ""
echo "=== [3/8] Building services ==="
"$BIN" --backend container -f "$COMPOSE_FILE" build

echo ""
echo "=== [4/8] Running up -d (with build check) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" up -d --build

# Brief sleep to allow services to initialize
sleep 3

echo ""
echo "=== [5/8] Listing status (ps) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" ps

echo ""
echo "=== [6/8] Testing Shared Resources (Bind Mount & Exec) ==="
echo "Writing file to shared host mount from container 'web'..."
"$BIN" --backend container -f "$COMPOSE_FILE" exec web sh -c "echo 'hello from shared host directory mount!' > /srv/hello.txt"

echo "Reading file from container 'caddy' via host port 8083..."
curl -s http://127.0.0.1:8083/hello.txt

echo ""
echo "=== [7/8] Fetching container logs (logs) ==="
echo "--- Logs for web (Python HTTP server traffic log) ---"
"$BIN" --backend container -f "$COMPOSE_FILE" logs web

echo ""
echo "=== [8/8] Tearing down resources and volumes (down -v) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" down -v

# Cleanup binary and shared files
rm -rf "$BIN" "$SHARED_DIR"
echo ""
echo "=== Business Flow Check: Complete ==="
