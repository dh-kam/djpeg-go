#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_DJPEG="${GO_DJPEG:-/tmp/djpeg_go_perf}"

go build -o "$GO_DJPEG" "$ROOT/cmd/djpeg"
exec "$ROOT/scripts/perf_compare.py" --go-djpeg "$GO_DJPEG" "$@"
