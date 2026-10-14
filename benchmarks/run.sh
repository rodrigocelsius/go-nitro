#!/usr/bin/env bash
# Reproducible HTTP benchmark: server pinned to CPU 0, load generator to CPU 1.
# Usage: ./run.sh [duration] [connections]   (needs `taskset`, Linux)
set -euo pipefail
cd "$(dirname "$0")"
DUR=${1:-5s}; CONNS=${2:-64}; PORT=${PORT:-9101}
go build -o /tmp/nitro-bench-server ./server
go build -o /tmp/nitro-bench-load ./load
printf '%-16s %-12s %10s %12s %12s\n' framework path req/s p50 p99
for fw in fasthttp fiber nitro-bare nitro nitro-defaults gin echo chi nethttp; do
  taskset -c 0 /tmp/nitro-bench-server -fw "$fw" -addr 127.0.0.1:$PORT >/dev/null 2>&1 &
  pid=$!; sleep 1.5
  for spec in "/ping GET" "/users/42 GET" "/echo POST"; do
    set -- $spec; body=""; [ "$2" = POST ] && body='{"name":"Ana","email":"a@x.com"}'
    out=$(taskset -c 1 /tmp/nitro-bench-load -addr 127.0.0.1:$PORT -path "$1" -m "$2" -body "$body" -c "$CONNS" -d "$DUR" | tail -1)
    # load prints: <req/s> <errors> <p50> <p99> <p99.9>
    set -- "$1" $out
    printf '%-16s %-12s %10s %12s %12s\n' "$fw" "$1" "$2" "$4" "$5"
  done
  kill $pid; wait $pid 2>/dev/null || true
done
