#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

selection=$(go list -m -f '{{if or (eq .Path "github.com/gotthboard/gotth-extensions") (eq .Path "github.com/gotthboard/gotth-sdk")}}{{.Path}} {{.Version}}{{with .Replace}} replace={{.Path}}@{{.Version}}{{end}}{{end}}' all | sed '/^$/d' | sort)
expected='github.com/gotthboard/gotth-extensions v1.0.0
github.com/gotthboard/gotth-sdk v1.0.0'
[ "$selection" = "$expected" ] || {
    printf 'unexpected foundation/SDK selection:\n%s\n' "$selection" >&2
    exit 1
}

if go list -m -f '{{if .Replace}}{{.Path}} => {{.Replace.Path}}{{end}}' all | grep -q .; then
    echo 'module replacement is forbidden' >&2
    exit 1
fi
grep -Fqx 'github.com/gotthboard/gotth-extensions v1.0.0 h1:lKXOydN/7+Mq9W2juqnWUXNJiirpYR6F3KPXYV55Hho=' go.sum
grep -Fqx 'github.com/gotthboard/gotth-sdk v1.0.0 h1:RmjRs/l+H3KMDmnimZs2OL8ExCchT/opVO5tDdjb4EA=' go.sum
go mod download github.com/gotthboard/gotth-extensions@v1.0.0 github.com/gotthboard/gotth-sdk@v1.0.0

extensions_dir=$(go list -m -f '{{.Dir}}' github.com/gotthboard/gotth-extensions)
sdk_dir=$(go list -m -f '{{.Dir}}' github.com/gotthboard/gotth-sdk)
source=proto/gotth/extensions/v1/control.proto
source_sha=846dbdd487e9688a338c7885478e1296456f994813330ecccdc3f4b225f10cfe
descriptor_sha=62a549b29a030f0decb46b2b1c2f382bbbc75430ba0b3d7c729bd21528b505cf

printf '%s  %s\n' "$source_sha" "$extensions_dir/$source" | sha256sum --check --status
printf '%s  %s\n' "$source_sha" "$sdk_dir/$source" | sha256sum --check --status
cmp "$extensions_dir/$source" "$sdk_dir/$source"
[ "$(cat "$extensions_dir/compat/v1/control-descriptor.sha256")" = "$descriptor_sha" ]
python3 - "$sdk_dir/proto/gotth/extensions/v1/source.json" "$source_sha" "$descriptor_sha" <<'PY'
import json, pathlib, sys
path, source_sha, descriptor_sha = sys.argv[1:]
value = json.loads(pathlib.Path(path).read_text())
want = {
    "repository": "https://git.dannyhunn.com/gotthboard/gotth-extensions",
    "commit": "32d13573a18622c16c57d5ef9c93d4874353885c",
    "tree_oid": "56a8f3532d1678835228fb8e19e36e34cdda057c",
    "path": "proto/gotth/extensions/v1/control.proto",
    "source_sha256": source_sha,
    "descriptor_sha256": descriptor_sha,
}
if value != want:
    raise SystemExit(f"unexpected SDK source binding: {value!r}")
PY

if find proto/gotth/extensions/v1 -type f -name 'control*' -print -quit 2>/dev/null | grep -q .; then
    echo 'repository-local extension control copy is forbidden' >&2
    exit 1
fi
if rg -n 'github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/extensions/v1' --glob '*.go' . >/dev/null; then
    echo 'repository-local extension control import is forbidden' >&2
    exit 1
fi
printf '%s\n' 'published foundation/SDK source binding verified; no local control copy'
