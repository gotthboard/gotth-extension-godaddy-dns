# v1.0.0 release and operations guide

## Release boundary

A successful verification run constructs deterministic release files; it does
not create a tag, publish to Forgejo, deploy the process, read a production
credential, or mutate DNS. Those remain explicit operator actions.

The release set has four deterministic files:

- `gotth-extension-godaddy-dns-1.0.0-linux-amd64.tar.gz`;
- `gotth-extension-godaddy-dns-1.0.0-linux-amd64.spdx.json`;
- `gotth-extension-godaddy-dns-1.0.0-linux-amd64.provenance.json`;
- `SHA256SUMS`, using basename-only entries for the three files above.

The archive contains exactly one top-level directory with the static executable,
`manifest.json`, `configuration-metadata.json`, the project and linked
dependency license texts in `LICENSE`, and `SHA256SUMS`. The SPDX 2.3
sidecar identifies the archive checksum, Git source identity embedded in the
binary, and the exact linked Go module versions and licenses. The deterministic
provenance binds the archive and SBOM hashes to the clean commit and tree,
`v1.0.0`, exact toolchain binary, `GOEXPERIMENT`, target, build flags, and
`SOURCE_DATE_EPOCH`. These sidecars remain outside the archive so the five-file
runtime artifact contract remains unchanged.

## Build and verify

Use the exact toolchain pinned in `release/toolchain.env`
(`go1.27.3-X:nodwarf5`, `GOEXPERIMENT=nodwarf5`, `GOAMD64=v1`, and its
Go executable SHA-256) and a clean checkout of the exact source commit. Publication must use
the clean-tree guard:

```sh
make verify
out=$(mktemp -d)
./scripts/build-artifact.sh 1.0.0 "$out"
```

Before publication, independently verify:

```sh
name=gotth-extension-godaddy-dns-1.0.0-linux-amd64
tar -tzf "$out/$name.tar.gz"
tmp=$(mktemp -d)
tar -xzf "$out/$name.tar.gz" -C "$tmp"
(cd "$tmp/$name" && sha256sum -c SHA256SUMS)
"$tmp/$name/gotth-extension-godaddy-dns" --version
go version -m "$tmp/$name/gotth-extension-godaddy-dns"
(cd "$out" && sha256sum -c SHA256SUMS)
cat "$out/$name.provenance.json"
```

The builder rejects a dirty tree by default.
`GOTTH_EXTENSION_ALLOW_DIRTY=1` exists only for reproducibility testing of an
uncommitted candidate; its output has `vcs.modified=true` and is not publishable.
The version must be exactly `1.0.0`; the binary build information must contain
the intended 40-hex Git revision, exact `GOEXPERIMENT`, and
`vcs.modified=false`; the provenance must name the same commit and commit tree
and bind the archive and SBOM SHA-256 values. Run
`scripts/check-dependency-licenses.sh` to prove that the linked module set,
module sums, upstream license files, notice, checked inventory, and distributed
license bundle still agree. `scripts/check-secrets.sh` verifies the retained
GoDaddy schema digest before applying the path-scoped Gitleaks allowlist for
that exact official snapshot; every other repository file uses the default
Gitleaks rules.

## Runtime contract

Install the extracted directory immutably. Run the executable as a dedicated
unprivileged account. Supply only canonical absolute paths through:

- `GOTTH_EXTENSION_CONFIG_FILE`;
- `GOTTH_EXTENSION_BINDING_FILE`;
- `GOTTH_EXTENSION_SERVICE_TOKEN_FILE`;
- `GOTTH_EXTENSION_SECRET_DIR`;
- `GOTTH_EXTENSION_SOCKET`.

The files and directories must satisfy the ownership and mode rules in
`docs/implementation-spec.md`. The socket must not already exist and must be
inside a private directory. The GoDaddy PAT belongs only in
`$GOTTH_EXTENSION_SECRET_DIR/godaddy.pat`; never place it in a unit file,
argument, environment value, manifest, or log.

The controller must enforce the manifest/grant/session binding and must be the
sole writer for a zone while a mutation is in flight. GoDaddy provides no
conditional record-write primitive, so the extension detects but cannot prevent
an external writer racing the precondition and write. API reconciliation is not
proof of authoritative-DNS propagation.

## Upgrade and rollback

For an upgrade, stop the old process, retain its immutable artifact and runtime
binding, verify the new archive and SBOM, generate a binding for the new exact
manifest digest, and start the new binary on an absent socket path. Do not reuse
a binding across manifest versions.

Binary rollback means stopping the current process and restoring the previously
verified immutable artifact and its matching binding. DNS mutation rollback is
separate and always requires a new host-approved operation against current
state:

- create inverse: delete the exact observed new record ID;
- replace inverse: restore the returned previous record at the same ID;
- delete inverse: recreate the returned previous logical record and accept a new
  provider record ID.

Never retry an unknown mutation blindly. Observe current state, reconcile the
recorded operation ID and exact before/after evidence, then authorize either an
inverse or a new desired-state operation.

## Deferred beyond v1.0.0

Registrar operations, DNSSEC, nameserver delegation, forwarding, PTR/rDNS,
ACME behavior, public-DNS propagation decisions, batch mutation, arbitrary HTTP,
and in-process plugins remain outside v1.0.0. Tags, hosted release objects,
production credentials, live-zone acceptance, and deployment are operational
actions requiring separate authorization; they are not hidden inside source
admission.
