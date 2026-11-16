#!/bin/sh
# Local 960x540 page for live tests of the web build; build it first (sh tools/web.sh). Usage: sh tools/serve.sh [-addr host:port]
cd "$(dirname "$0")/.." && exec go run tools/serve.go "$@"
