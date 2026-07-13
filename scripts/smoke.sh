#!/usr/bin/env bash
# End-to-end functionality check. By default this builds bianpai and drives
# `up`/`down`/`ps` against every usecase stack using a fake `container` CLI that
# records argv. Set SMOKE_MODE=real to use the real Apple `container` CLI and
# verify each stack's exposed application endpoint.
#
# Usage: scripts/smoke.sh
set -euo pipefail

cd "$(dirname "$0")/.."

tmp="$(mktemp -d)"

bin="$tmp/bianpai"
log="$tmp/argv.log"
stubdir="$tmp/stub"
mode="${SMOKE_MODE:-fake}"
current_file=""
cleanup() {
  if [ "$mode" = "real" ] && [ -n "$current_file" ] && [ -x "$bin" ]; then
    "$bin" --backend container -f "$current_file" down >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT
mkdir -p "$stubdir"

if [ "$mode" = "fake" ]; then
  # Fake `container` CLI: log every invocation, answer `list` with the string
  # status shape emitted by the real CLI, and succeed everywhere else.
  cat >"$stubdir/container" <<EOF
#!/usr/bin/env bash
echo "\$*" >> "$log"
if [ "\${1:-}" = "list" ]; then
  echo '[{"id":"smoke_web_1","configuration":{"labels":{"com.bianpai.project":"smoke"},"image":{"reference":"smoke"}},"status":"running"}]'
fi
exit 0
EOF
  chmod +x "$stubdir/container"
elif [ "$mode" = "real" ]; then
  command -v container >/dev/null || { echo "container CLI is required for SMOKE_MODE=real" >&2; exit 1; }
  command -v curl >/dev/null || { echo "curl is required for SMOKE_MODE=real" >&2; exit 1; }
else
  echo "unknown SMOKE_MODE: $mode" >&2
  exit 1
fi

echo "building bianpai..."
go build -o "$bin" .

if [ "$mode" = "fake" ]; then
  export PATH="$stubdir:$PATH"
fi

fail=0
pass=0
check() { # label, pattern
  if grep -qF -- "$2" "$log"; then
    printf '  ok   %s\n' "$1"; pass=$((pass + 1))
  else
    printf '  FAIL %s (no match: %s)\n' "$1" "$2"; fail=$((fail + 1))
  fi
}

check_http() { # label, url, pattern
  label="$1"
  url="$2"
  pattern="$3"
  last_body=""
  for _ in $(seq 1 30); do
    body="$(curl --fail --silent --show-error --max-time 2 "$url" 2>/dev/null || true)"
    last_body="$body"
    if printf '%s' "$body" | grep -qF -- "$pattern"; then
      printf '  ok   %s\n' "$label"
      pass=$((pass + 1))
      return
    fi
    sleep 2
  done
  printf '  FAIL %s (no response matching %s from %s)\n' "$label" "$pattern" "$url"
  if [ -n "$last_body" ]; then
    printf '  Last response body:\n%s\n' "$last_body"
  else
    printf '  Last response body: (empty)\n'
  fi
  fail=$((fail + 1))
}

check_exec() { # label, compose-file, service, command...
  label="$1"
  file="$2"
  service="$3"
  shift 3
  last_err=""
  for _ in $(seq 1 30); do
    tmp_exec="$(mktemp)"
    if "$bin" --backend container -f "$file" exec "$service" "$@" >"$tmp_exec" 2>&1; then
      rm -f "$tmp_exec"
      printf '  ok   %s\n' "$label"
      pass=$((pass + 1))
      return
    fi
    last_err="$(cat "$tmp_exec")"
    rm -f "$tmp_exec"
    sleep 2
  done
  printf '  FAIL %s\n' "$label"
  if [ -n "$last_err" ]; then
    printf '  Last exec output:\n%s\n' "$last_err"
  else
    printf '  Last exec output: (empty)\n'
  fi
  fail=$((fail + 1))
}

check_real_access() { # stack-name
  case "$1" in
    caddy-host)
      check_http "app access caddy" "http://127.0.0.1:8083/" "bianpai caddy host"
      ;;
    multi-host)
      check_http "app access caddy (multi-host)" "http://127.0.0.1:8083/" "bianpai caddy multi-host"
      check_http "app access python (multi-host)" "http://127.0.0.1:8082/" "Directory listing"
      ;;
    etcd-single)
      check_http "app access etcd" "http://127.0.0.1:12379/version" "etcdserver"
      ;;
    postgres-host)
      check_exec "app access postgres" "$current_file" "db" pg_isready -U app -d app
      ;;
    python-host)
      check_http "app access python" "http://127.0.0.1:8082/" "Directory listing"
      ;;
  esac
}

real_dns_unsupported() { # stack-name
  case "$1" in
    etcd-cluster|postgres-adminer|python-valkey-caddy) return 0 ;;
    *) return 1 ;;
  esac
}

check_unsupported_service_dns() { # stack-name, compose-file
  output="$("$bin" --backend container -f "$2" up -d 2>&1)" && {
    printf '  FAIL unsupported service DNS was accepted\n'
    fail=$((fail + 1))
    "$bin" --backend container -f "$2" down >/dev/null 2>&1 || true
    return
  }
  if printf '%s' "$output" | grep -qF -- "does not support Compose service DNS"; then
    printf '  ok   rejects service DNS\n'
    pass=$((pass + 1))
  else
    printf '  FAIL unsupported error mismatch: %s\n' "$output"
    fail=$((fail + 1))
  fi
}

for dir in usecases/*/; do
  [ -f "$dir/compose.yaml" ] || continue
  name="$(basename "$dir")"
  project="$(grep -E '^name:' "$dir/compose.yaml" | head -1 | awk '{print $2}')"
  : >"$log"
  current_file="$dir/compose.yaml"

  echo "=== $name (project=$project) ==="
  if real_dns_unsupported "$name"; then
    check_unsupported_service_dns "$name" "$current_file"
    current_file=""
    continue
  fi

  up_out="$(mktemp)"
  if ! "$bin" --backend container -f "$current_file" up -d >"$up_out" 2>&1; then
    printf '  FAIL up -d failed:\n'
    cat "$up_out"
    rm -f "$up_out"
    fail=$((fail + 1))
    current_file=""
    continue
  fi
  rm -f "$up_out"

  if [ "$mode" = "real" ]; then
    check_real_access "$name"
    ps_out="$(mktemp)"
    if "$bin" --backend container -f "$current_file" ps >"$ps_out" 2>&1; then
      printf '  ok   ps\n'
      pass=$((pass + 1))
    else
      printf '  FAIL ps (non-zero exit):\n'
      cat "$ps_out"
      fail=$((fail + 1))
    fi
    rm -f "$ps_out"
    
    down_out="$(mktemp)"
    if ! "$bin" --backend container -f "$current_file" down >"$down_out" 2>&1; then
      printf '  warning: down failed:\n'
      cat "$down_out"
    fi
    rm -f "$down_out"
    current_file=""
    continue
  fi

  # one `run` per service, named <project>_<service>_1. Service names are the
  # 2-space-indented keys inside the top-level `services:` block only.
  services="$(awk '
    /^services:/ { in_svc=1; next }
    /^[a-zA-Z]/  { in_svc=0 }
    in_svc && /^  [a-z0-9_-]+:/ { gsub(/[ :]/, ""); print }
  ' "$dir/compose.yaml")"
  for svc in $services; do
    check "run $svc" "run --detach --name ${project}_${svc}_1"
  done
  if grep -qF -- "--network-alias" "$log"; then
    printf '  FAIL no network aliases (unsupported by Apple container)\n'
    fail=$((fail + 1))
  else
    printf '  ok   no network aliases\n'
    pass=$((pass + 1))
  fi
  # networks created with the project label
  check "network create" "network create --label com.bianpai.project=${project}"

  # `down` stops+deletes and removes the network
  : >"$log"
  "$bin" --backend container -f "$current_file" down >/dev/null 2>&1
  current_file=""
  check "down delete" "delete ${project}_"
  check "down network delete" "network delete ${project}_"

  # `ps` decodes the fake list JSON without error
  if "$bin" --backend container -f "$dir/compose.yaml" ps >/dev/null 2>&1; then
    printf '  ok   ps\n'; pass=$((pass + 1))
  else
    printf '  FAIL ps (non-zero exit)\n'; fail=$((fail + 1))
  fi
done

echo
echo "smoke: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
