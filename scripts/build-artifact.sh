#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then echo "usage: $0 <version> <output-directory>" >&2; exit 2; fi
version=$1
output=$2
if ! printf '%s\n' "$version" | grep -Eq '^1\.0\.0$|^1\.0\.0-(alpha|beta)\.[1-9][0-9]*$'; then echo "invalid release version" >&2; exit 2; fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
commit_epoch=$(git -C "$root" show -s --format=%ct HEAD)
epoch=${SOURCE_DATE_EPOCH:-$commit_epoch}
if [ "$epoch" != "$commit_epoch" ]; then echo "SOURCE_DATE_EPOCH must equal the source commit time" >&2; exit 1; fi
if [ "${GOTTH_EXTENSION_REQUIRE_CLEAN:-0}" != 0 ] && [ "${GOTTH_EXTENSION_REQUIRE_CLEAN:-0}" != 1 ]; then echo "GOTTH_EXTENSION_REQUIRE_CLEAN must be 0 or 1" >&2; exit 2; fi
if [ "${GOTTH_EXTENSION_REQUIRE_CLEAN:-0}" = 1 ] && [ -n "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" ]; then echo "release artifact build requires a clean source tree" >&2; exit 1; fi

mkdir -p "$output"
stage=$(mktemp -d "$output/.gotth-extension-godaddy-dns.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM
name="gotth-extension-godaddy-dns-$version-linux-amd64"
mkdir -p "$stage/$name"
(
cd "$root"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-s -w -buildid= -X github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns.BuildVersion=$version" \
  -o "$stage/$name/gotth-extension-godaddy-dns" ./cmd/gotth-extension-godaddy-dns
go run ./cmd/gotth-extension-godaddy-dns-manifest "$version" > "$stage/$name/manifest.json"
)
test "$("$stage/$name/gotth-extension-godaddy-dns" --version)" = "$version"
cp "$root/configuration-metadata.json" "$stage/$name/configuration-metadata.json"
cp "$root/LICENSE" "$stage/$name/LICENSE"
(cd "$stage/$name" && sha256sum LICENSE configuration-metadata.json gotth-extension-godaddy-dns manifest.json > SHA256SUMS)
chmod 0755 "$stage/$name" "$stage/$name/gotth-extension-godaddy-dns"
chmod g-s "$stage/$name"
chmod 0644 "$stage/$name/LICENSE" "$stage/$name/configuration-metadata.json" "$stage/$name/manifest.json" "$stage/$name/SHA256SUMS"
temporary="$stage/$name.tar.gz"
archive="$output/$name.tar.gz"
tar --format=ustar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - "$name" | gzip -n > "$temporary"
ln "$temporary" "$archive"
sync -d "$archive"
sync -d "$output"
printf '%s  %s\n' "$(sha256sum "$archive" | cut -d' ' -f1)" "$archive"
