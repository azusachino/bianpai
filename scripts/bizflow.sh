#!/bin/bash
set -euo pipefail

# Resolve project paths
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$DIR/.." && pwd)"

echo "=== Building bianpai ==="
go build -o "$PROJECT_DIR/bianpai-biz" "$PROJECT_DIR/main.go"
BIN="$PROJECT_DIR/bianpai-biz"

COMPOSE_FILE="$PROJECT_DIR/usecases/multi-host/compose.yaml"

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
echo "=== [4/8] Running up -d ==="
"$BIN" --backend container -f "$COMPOSE_FILE" up -d

# Brief sleep to allow services to initialize
sleep 2

echo ""
echo "=== [5/8] Listing status (ps) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" ps

echo ""
echo "=== [6/8] Executing command inside container (exec) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" exec web python -c "print('Python VM connection check: OK')"

echo ""
echo "=== [7/8] Fetching container logs (logs) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" logs web

echo ""
echo "=== [8/8] Tearing down resources (down) ==="
"$BIN" --backend container -f "$COMPOSE_FILE" down

# Cleanup binary
rm -f "$BIN"
echo ""
echo "=== Business Flow Check: Complete ==="
