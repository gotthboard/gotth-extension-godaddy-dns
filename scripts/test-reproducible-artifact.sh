#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
left=$(mktemp -d)
right=$(mktemp -d)
extract=$(mktemp -d)
hostile=$(mktemp -d)
dirty_marker="$root/.release-dirty-test"
[ ! -e "$dirty_marker" ]
cleanup() { rm -rf "$left" "$right" "$extract" "$hostile"; rm -f "$dirty_marker"; }
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
epoch=$(git -C "$root" show -s --format=%ct HEAD)
GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0 "$left" >/dev/null
GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0 "$right" >/dev/null
GOAMD64=v3 GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0 "$hostile" >/dev/null
name=gotth-extension-godaddy-dns-1.0.0-linux-amd64
cmp "$left/$name.tar.gz" "$right/$name.tar.gz"
cmp "$left/$name.spdx.json" "$right/$name.spdx.json"
cmp "$left/$name.provenance.json" "$right/$name.provenance.json"
cmp "$left/SHA256SUMS" "$right/SHA256SUMS"
cmp "$left/$name.tar.gz" "$hostile/$name.tar.gz"
cmp "$left/$name.spdx.json" "$hostile/$name.spdx.json"
cmp "$left/$name.provenance.json" "$hostile/$name.provenance.json"
cmp "$left/SHA256SUMS" "$hostile/SHA256SUMS"
(cd "$left" && sha256sum -c SHA256SUMS)
if grep -q '/' "$left/SHA256SUMS"; then echo "release checksums are not basename-portable" >&2; exit 1; fi
tar -xzf "$left/$name.tar.gz" -C "$extract"
test "$(stat -c %a "$extract/$name")" = 755
test "$("$extract/$name/gotth-extension-godaddy-dns" --version)" = 1.0.0
(cd "$extract/$name" && sha256sum -c SHA256SUMS)
grep -Fq 'github.com/gotthboard/gotth-sdk v1.0.0' "$extract/$name/LICENSE"
grep -Fq '"spdxVersion": "SPDX-2.3"' "$left/$name.spdx.json"
grep -Fq '"versionInfo": "1.0.0"' "$left/$name.spdx.json"
grep -Fq '"documentNamespace": "urn:gotth:sbom:gotth-extension-godaddy-dns:v1.0.0:' "$left/$name.spdx.json"
grep -Fq 'pkg:golang/github.com/gotthboard/gotth-extension-godaddy-dns@v1.0.0' "$left/$name.spdx.json"
grep -Fq "$(sha256sum "$left/$name.tar.gz" | cut -d' ' -f1)" "$left/$name.spdx.json"
grep -Fq '"schema": "urn:gotth:release-provenance:v1"' "$left/$name.provenance.json"
grep -Fq '"goVersion": "go1.27.3-X:nodwarf5"' "$left/$name.provenance.json"
grep -Fq '"goExperiment": "nodwarf5"' "$left/$name.provenance.json"
grep -Fq '"goAmd64": "v1"' "$left/$name.provenance.json"
grep -Fq "$(git -C "$root" rev-parse HEAD)" "$left/$name.provenance.json"
grep -Fq "$(git -C "$root" rev-parse 'HEAD^{tree}')" "$left/$name.provenance.json"
grep -Fq "$(sha256sum "$left/$name.tar.gz" | cut -d' ' -f1)" "$left/$name.provenance.json"
grep -Fq "$(sha256sum "$left/$name.spdx.json" | cut -d' ' -f1)" "$left/$name.provenance.json"
go version -m "$extract/$name/gotth-extension-godaddy-dns" | grep -Eq '^[[:space:]]+build[[:space:]]+vcs.revision=[0-9a-f]{40}$'
bad_inventory="$extract/dependencies.tsv"
sed '/github.com\/gotthboard\/gotth-sdk/d' "$root/release/dependencies.tsv" > "$bad_inventory"
if (cd "$root" && SOURCE_DATE_EPOCH="$epoch" go run ./cmd/gotth-extension-godaddy-dns-sbom "$extract/$name/gotth-extension-godaddy-dns" "$left/$name.tar.gz" 1.0.0 "$bad_inventory" "$extract/bad.spdx.json") >/dev/null 2>&1; then echo "incomplete dependency inventory was accepted" >&2; exit 1; fi
if GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0-alpha.2 "$right/prerelease" >/dev/null 2>&1; then echo "prerelease version was accepted" >&2; exit 1; fi
: > "$dirty_marker"
if SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0 "$right/dirty" >/dev/null 2>&1; then echo "dirty release source was accepted" >&2; exit 1; fi
rm -f "$dirty_marker"
before_archive=$(sha256sum "$right/$name.tar.gz" | cut -d' ' -f1)
before_sbom=$(sha256sum "$right/$name.spdx.json" | cut -d' ' -f1)
before_provenance=$(sha256sum "$right/$name.provenance.json" | cut -d' ' -f1)
before_checksums=$(sha256sum "$right/SHA256SUMS" | cut -d' ' -f1)
if GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH="$epoch" "$root/scripts/build-artifact.sh" 1.0.0 "$right" >/dev/null 2>&1; then echo "existing release output was replaced" >&2; exit 1; fi
test "$before_archive" = "$(sha256sum "$right/$name.tar.gz" | cut -d' ' -f1)"
test "$before_sbom" = "$(sha256sum "$right/$name.spdx.json" | cut -d' ' -f1)"
test "$before_provenance" = "$(sha256sum "$right/$name.provenance.json" | cut -d' ' -f1)"
test "$before_checksums" = "$(sha256sum "$right/SHA256SUMS" | cut -d' ' -f1)"
if GOTTH_EXTENSION_ALLOW_DIRTY=1 SOURCE_DATE_EPOCH=$((epoch + 1)) "$root/scripts/build-artifact.sh" 1.0.0 "$right/epoch" >/dev/null 2>&1; then echo "mismatched source epoch was accepted" >&2; exit 1; fi
