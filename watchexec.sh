#!/usr/bin/env bash
set -euo pipefail
exec watchexec -r -e go -- go run ./cmd/cfntop "$@"
