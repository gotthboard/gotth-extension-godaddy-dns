#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
./scripts/check-source-boundary.sh >/dev/null
exec go run github.com/zricethezav/gitleaks/v8@v8.30.1 dir . --config .gitleaks.toml --no-banner --redact --exit-code 1
