#!/usr/bin/env bash
# Re-run the Go test suite N times to catch flakes / ordering bugs.
#
# Usage:
#   scripts/test.sh            # run the whole suite once
#   scripts/test.sh 20         # run it 20 times (-count=20, no cache)
#   scripts/test.sh 20 ./internal/compose/...   # 20x, scoped to one package
#   COUNT=50 scripts/test.sh   # count via env
#
# Any extra args after the count are passed straight to `go test`.
set -euo pipefail

cd "$(dirname "$0")/.."

count="${1:-${COUNT:-1}}"
if [[ "${1:-}" =~ ^[0-9]+$ ]]; then
  shift
fi

pkgs=("$@")
if [[ ${#pkgs[@]} -eq 0 ]]; then
  pkgs=("./...")
fi

echo "running ${pkgs[*]} x${count}"
go test -count="${count}" "${pkgs[@]}"
