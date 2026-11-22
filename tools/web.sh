#!/bin/sh
# Web build for itch.io: web/game.wasm (gzipped to game.wasm.gz) + web/wasm_exec.js, zipped with web/index.html into web/ghosthotel-web.zip
# (upload it as an HTML game, viewport 960x540). Run from anywhere: sh tools/web.sh
# sh tools/web.sh prod: the production build, for itch.io: no dev flags, ?args=, ?story or FPS readout (game/prod.go).
set -e
cd "$(dirname "$0")/.."
tags=""
[ "$1" = prod ] && tags="-tags prod"
GOOS=js GOARCH=wasm GOMAXPROCS=2 go build $tags -trimpath -ldflags="-s -w" -o web/game.wasm .
root=$(go env GOROOT)
gzip -9 -nf -k web/game.wasm
cp "$root/lib/wasm/wasm_exec.js" web/ 2>/dev/null || cp "$root/misc/wasm/wasm_exec.js" web/
py=$(command -v python || command -v python3)
"$py" -c 'import sys, zipfile, os
with zipfile.ZipFile(sys.argv[1], "w", zipfile.ZIP_DEFLATED) as z:
    for f in sys.argv[2:]: z.write(f, os.path.basename(f))' web/ghosthotel-web.zip web/index.html web/wasm_exec.js web/game.wasm.gz
ls -l web/game.wasm web/game.wasm.gz web/ghosthotel-web.zip
