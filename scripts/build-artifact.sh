#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then echo "usage: $0 <version> <output-directory>" >&2; exit 2; fi
version=$1
output=$2
if [ "$version" != 1.0.0 ]; then echo "release version must be 1.0.0" >&2; exit 2; fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$root/release/toolchain.env"
go_bin=$(command -v go)
actual_go_version=$("$go_bin" version | awk '{print $3}')
actual_go_sha=$(sha256sum "$go_bin" | cut -d' ' -f1)
if [ "$actual_go_version" != "$GO_VERSION" ] || [ "$actual_go_sha" != "$GO_BINARY_SHA256" ]; then
    echo "release toolchain identity mismatch" >&2
    exit 1
fi
release_go() {
    env GOENV=off GOFLAGS= GOTOOLCHAIN=local GOEXPERIMENT="$GOEXPERIMENT" GOAMD64="$GOAMD64" "$go_bin" "$@"
}
if [ "$(GOENV=off GOEXPERIMENT="$GOEXPERIMENT" "$go_bin" env GOEXPERIMENT)" != "$GOEXPERIMENT" ]; then
    echo "release GOEXPERIMENT mismatch" >&2
    exit 1
fi

commit=$(git -C "$root" rev-parse HEAD)
tree=$(git -C "$root" rev-parse 'HEAD^{tree}')
commit_epoch=$(git -C "$root" show -s --format=%ct HEAD)
epoch=${SOURCE_DATE_EPOCH:-$commit_epoch}
if [ "$epoch" != "$commit_epoch" ]; then echo "SOURCE_DATE_EPOCH must equal the source commit time" >&2; exit 1; fi
if [ "${GOTTH_EXTENSION_ALLOW_DIRTY:-0}" != 0 ] && [ "${GOTTH_EXTENSION_ALLOW_DIRTY:-0}" != 1 ]; then echo "GOTTH_EXTENSION_ALLOW_DIRTY must be 0 or 1" >&2; exit 2; fi
modified=false
if [ -n "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" ]; then modified=true; fi
if [ "${GOTTH_EXTENSION_ALLOW_DIRTY:-0}" != 1 ] && [ "$modified" = true ]; then echo "release artifact build requires a clean source tree" >&2; exit 1; fi

mkdir -p "$output"
name="gotth-extension-godaddy-dns-$version-linux-amd64"
archive="$output/$name.tar.gz"
sbom="$output/$name.spdx.json"
provenance="$output/$name.provenance.json"
checksums="$output/SHA256SUMS"
for target in "$archive" "$sbom" "$provenance" "$checksums"; do
    if [ -e "$target" ]; then echo "release output already exists" >&2; exit 1; fi
done
stage=$(mktemp -d "$output/.gotth-extension-godaddy-dns.XXXXXX")
archive_published=0
sbom_published=0
provenance_published=0
checksums_published=0
complete=0
cleanup() {
    rm -rf "$stage"
    if [ "$complete" != 1 ]; then
        [ "$archive_published" != 1 ] || rm -f "$archive"
        [ "$sbom_published" != 1 ] || rm -f "$sbom"
        [ "$provenance_published" != 1 ] || rm -f "$provenance"
        [ "$checksums_published" != 1 ] || rm -f "$checksums"
    fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
mkdir -p "$stage/$name"
(
cd "$root"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64="$GOAMD64" release_go build -trimpath -buildvcs=true \
  -ldflags "-s -w -buildid= -X github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns.BuildVersion=$version" \
  -o "$stage/$name/gotth-extension-godaddy-dns" ./cmd/gotth-extension-godaddy-dns
release_go run ./cmd/gotth-extension-godaddy-dns-manifest "$version" > "$stage/$name/manifest.json"
)
test "$("$stage/$name/gotth-extension-godaddy-dns" --version)" = "$version"
buildinfo=$(release_go version -m "$stage/$name/gotth-extension-godaddy-dns")
printf '%s\n' "$buildinfo" | grep -Fq "$(printf '\tbuild\tvcs.revision=%s' "$commit")"
printf '%s\n' "$buildinfo" | grep -Fq "$(printf '\tbuild\tvcs.modified=%s' "$modified")"
printf '%s\n' "$buildinfo" | grep -Fq "$(printf '\tbuild\tGOEXPERIMENT=%s' "$GOEXPERIMENT")"
printf '%s\n' "$buildinfo" | grep -Fq "$(printf '\tbuild\tGOAMD64=%s' "$GOAMD64")"
if [ "${GOTTH_EXTENSION_ALLOW_DIRTY:-0}" != 1 ] && [ "$modified" != false ]; then
    echo "publishable artifact must embed vcs.modified=false" >&2
    exit 1
fi
cp "$root/configuration-metadata.json" "$stage/$name/configuration-metadata.json"
cat "$root/LICENSE" "$root/THIRD_PARTY_LICENSES.txt" > "$stage/$name/LICENSE"
(cd "$stage/$name" && sha256sum LICENSE configuration-metadata.json gotth-extension-godaddy-dns manifest.json > SHA256SUMS)
chmod 0755 "$stage/$name" "$stage/$name/gotth-extension-godaddy-dns"
chmod g-s "$stage/$name"
chmod 0644 "$stage/$name/LICENSE" "$stage/$name/configuration-metadata.json" "$stage/$name/manifest.json" "$stage/$name/SHA256SUMS"
temporary="$stage/$name.tar.gz"
temporary_sbom="$stage/$name.spdx.json"
temporary_provenance="$stage/$name.provenance.json"
temporary_checksums="$stage/SHA256SUMS"
tar --format=ustar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - "$name" | gzip -n > "$temporary"
(
cd "$root"
SOURCE_DATE_EPOCH="$epoch" release_go run ./cmd/gotth-extension-godaddy-dns-sbom "$stage/$name/gotth-extension-godaddy-dns" "$temporary" "$version" "$root/release/dependencies.tsv" "$temporary_sbom"
SOURCE_DATE_EPOCH="$epoch" GOTTH_GO_BINARY_SHA256="$GO_BINARY_SHA256" release_go run ./cmd/gotth-extension-godaddy-dns-provenance "$stage/$name/gotth-extension-godaddy-dns" "$temporary" "$temporary_sbom" "$version" "$commit" "$tree" "$modified" "$temporary_provenance"
)
(cd "$stage" && sha256sum "$name.tar.gz" "$name.spdx.json" "$name.provenance.json" > SHA256SUMS)
if [ "${GOTTH_EXTENSION_ALLOW_DIRTY:-0}" != 1 ] && [ -n "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" ]; then
    echo "release source changed during build" >&2
    exit 1
fi
ln "$temporary" "$archive"; archive_published=1
ln "$temporary_sbom" "$sbom"; sbom_published=1
ln "$temporary_provenance" "$provenance"; provenance_published=1
ln "$temporary_checksums" "$checksums"; checksums_published=1
sync -d "$archive"
sync -d "$sbom"
sync -d "$provenance"
sync -d "$checksums"
sync -d "$output"
complete=1
(cd "$output" && cat SHA256SUMS)
