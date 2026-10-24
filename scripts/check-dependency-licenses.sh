#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
inventory=release/dependencies.tsv
notices_sha=e75dcb986d23ece8e52511916915d7429383d43abd7921892ac57d6f47a4cb74
printf '%s  %s\n' "$notices_sha" THIRD_PARTY_LICENSES.txt | sha256sum --check --status

expected=$(awk -F '\t' '!/^#/ && NF {print $1 " " $2}' "$inventory" | sort)
actual=$(go list -deps -f '{{with .Module}}{{if ne .Path "github.com/gotthboard/gotth-extension-godaddy-dns"}}{{.Path}} {{.Version}}{{end}}{{end}}' ./cmd/gotth-extension-godaddy-dns | sed '/^$/d' | sort -u)
[ "$actual" = "$expected" ] || {
    printf 'linked dependency inventory mismatch\nexpected:\n%s\nactual:\n%s\n' "$expected" "$actual" >&2
    exit 1
}

while IFS="$(printf '\t')" read -r module version sum license_id license_sha notice_sha; do
    case "$module" in ''|'#'*) continue ;; esac
    case "$license_id" in MIT|BSD-3-Clause|Apache-2.0) ;; *) echo "invalid license identifier for $module" >&2; exit 1 ;; esac
    selected=$(go list -m -f '{{.Version}} {{.Sum}}' "$module")
    [ "$selected" = "$version $sum" ] || { echo "module selection mismatch for $module" >&2; exit 1; }
    dir=$(go list -m -f '{{.Dir}}' "$module")
    license=$(find "$dir" -maxdepth 1 -type f -iname 'LICENSE*' | head -1)
    [ -n "$license" ]
    printf '%s  %s\n' "$license_sha" "$license" | sha256sum --check --status
    if [ "$notice_sha" != - ]; then
        notice=$(find "$dir" -maxdepth 1 -type f -iname 'NOTICE*' | head -1)
        [ -n "$notice" ]
        printf '%s  %s\n' "$notice_sha" "$notice" | sha256sum --check --status
    fi
done < "$inventory"
printf '%s\n' 'linked dependency versions, licenses, notices, and inventory verified'
