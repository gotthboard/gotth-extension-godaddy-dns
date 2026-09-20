#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
left=$(mktemp -d)
right=$(mktemp -d)
trap 'rm -rf "$left" "$right"' EXIT HUP INT TERM
epoch=$(git -C "$root" show -s --format=%ct HEAD)
SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0-alpha.1 "$left" >/dev/null
SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0-alpha.1 "$right" >/dev/null
name=gotth-extension-godaddy-dns-1.0.0-alpha.1-linux-amd64
cmp "$left/$name.tar.gz" "$right/$name.tar.gz"
extract=$(mktemp -d)
trap 'rm -rf "$left" "$right" "$extract"' EXIT HUP INT TERM
tar -xzf "$left/$name.tar.gz" -C "$extract"
test "$(stat -c %a "$extract/$name")" = 755
test "$("$extract/$name/gotth-extension-godaddy-dns" --version)" = 1.0.0-alpha.1
(cd "$extract/$name" && sha256sum -c SHA256SUMS)
before=$(sha256sum "$right/$name.tar.gz" | cut -d' ' -f1)
if SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0-alpha.1 "$right" >/dev/null 2>&1; then echo "existing release artifact was replaced" >&2; exit 1; fi
test "$before" = "$(sha256sum "$right/$name.tar.gz" | cut -d' ' -f1)"
if SOURCE_DATE_EPOCH=$((epoch + 1)) "$root/scripts/build-artifact.sh" 1.0.0-alpha.1 "$right/epoch" >/dev/null 2>&1; then echo "mismatched source epoch was accepted" >&2; exit 1; fi
