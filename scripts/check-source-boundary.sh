#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
(cd contracts && sha256sum --check --status SHA256SUMS) || {
    echo 'GoDaddy contract snapshot digest mismatch' >&2
    exit 1
}
production='internal/godaddydns cmd/gotth-extension-godaddy-dns'
if rg -n 'os/exec|plugin\.|ProxyFromEnvironment|http\.Default(Client|Transport)|InsecureSkipVerify' $production --glob '*.go' --glob '!*_test.go'; then
    echo 'forbidden production mechanism detected' >&2
    exit 1
fi
hosts=$(rg -l 'api\.godaddy\.com|api\.ote-godaddy\.com' internal/godaddydns --glob '*.go' --glob '!*_test.go' | sort -u)
[ "$hosts" = internal/godaddydns/client_endpoint.go ] || {
    printf 'provider hosts escaped fixed client: %s\n' "$hosts" >&2
    exit 1
}
compiled=$(go list -f '{{range .GoFiles}}{{println .}}{{end}}' ./internal/godaddydns)
echo "$compiled" | grep -qx 'client_endpoint.go'
if echo "$compiled" | grep -q 'processintegration'; then
    echo 'process-integration endpoint seam entered production build' >&2
    exit 1
fi
printf '%s\n' 'production source boundary verified'
