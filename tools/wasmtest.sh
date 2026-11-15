#!/bin/sh
# Runs Go tests as wasm in Node (the itch.io build's runtime), with a browser stub (tools/wasm/dom_shim.js).
# Usage: sh tools/wasmtest.sh [pkg ...] [-- go test flags]   e.g. sh tools/wasmtest.sh ./world ./game -- -test.run Path -test.bench Update
# Flags are the compiled test binary's: -test.run, -test.bench, -test.v, -test.timeout.
# Source-scanning guard tests skip themselves under GOOS=js (Node can't open directories on Windows).
set -e
root=$(cd "$(dirname "$0")/.." && pwd)
pkgs=""
while [ $# -gt 0 ] && [ "$1" != "--" ]; do pkgs="$pkgs $1"; shift; done
[ "$1" = "--" ] && shift
[ -n "$pkgs" ] || pkgs=$(cd "$root" && go list ./... | grep -v /tools/ | sed "s|^$(cd "$root" && go list -m)|.|")
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
export GOOS=js GOARCH=wasm
export WASM_EXEC_NODE="$(go env GOROOT)/lib/wasm/wasm_exec_node.js"
[ -f "$WASM_EXEC_NODE" ] || WASM_EXEC_NODE="$(go env GOROOT)/misc/wasm/wasm_exec_node.js"
node=$(command -v node || echo "/c/Program Files/nodejs/node")
shim=$(cd "$root/tools/wasm" && pwd -W 2>/dev/null || pwd)/dom_shim.js
status=0
for p in $pkgs; do
  bin="$out/$(echo "$p" | tr -c 'a-zA-Z0-9\n' _).test.wasm"
  (cd "$root" && go test -c -o "$bin" "$p") 2>&1 | grep -v "no test files" || true
  [ -f "$bin" ] || continue
  echo "== $p"
  (cd "$root/$p" && "$node" --stack-size=8000 "$shim" "$bin" "$@") || status=1
done
exit $status
