# Verification status

## Final trust-boundary repair — 2026-10-24

The source on `workflow/feature/v1-release` now binds and enforces the
exact negotiated DNS interface, validates printable-ASCII service tokens,
applies RFC 4122 version/variant semantics to RPC request IDs without changing
foundation instance-ID grammar, and exercises real compiled-binary Handshake,
ObserveZone, and CreateRecord calls against a fake TLS provider. Focused,
compiled-process, security, source-boundary, reproducibility, and clean release
gates were rerun against the repaired source.

Exact commit/tree/toolchain identities, logs, source manifests, clean
`vcs.modified=false` archive/SBOM/provenance/checksums, and the resealed
evidence root are stored under
`.artifacts/gotth-extension-godaddy-v1-release-final-20261024`. The source
is committed, but there is no tag or hosted release; no push, credential, live
provider request, DNS mutation, or deployment occurred.

## Production v1.0.0 preparation — 2026-10-23

The initial source-preparation run was based on
`c860b030fd7ff8297a244c1da1f7d57844bcde5e` before a release commit or tag.
Local focused gates passed. The remediated source then passed isolated
development full tests, ten focused fake-GoDaddy repetitions, full race, 20
repeats, 20 shuffled race repeats, three 10-second fuzz targets, 78.7% statement
coverage, vet, build, compiled-process integration, published dependency and
control binding, linked-license and retained-contract verification,
deterministic archive/SPDX/provenance/checksum reproduction, `govulncheck`,
scoped `gitleaks`, format, and temporary-index diff checks including untracked
files. A final affected rerun repeated full, focused, race, provider-JSON fuzz,
coverage, release-set, secret, format, and diff gates.

The following paragraph records the historical pre-commit state of that run;
it is superseded by the final repair status above. The exact raw logs and
candidate seal are under
`.artifacts/gotth-extension-godaddy-v1-release-20261023`. The deterministic
candidate archive, SPDX, provenance, and checksum set retained there are
verification evidence only: because commit authority is excluded, their
embedded build information
honestly reports `vcs.modified=true`. A publishable v1.0.0 release artifact
remains blocked on an authorized commit of the sealed tree followed by the
clean-source build gate; tag, hosted release, live credentials, DNS changes, and
deployment remain separately blocked.

## Published Extensions V1 adoption candidate — 2026-10-23

The candidate selects exact published `gotth-extensions v1.0.0` and
`gotth-sdk v1.0.0` without replacement. `scripts/check-published-control.sh`
checks both published control sources, the SDK source binding, the foundation
descriptor pin, and absence of a repository-local control copy.
`TestPublishedControlDescriptorBinding` checks the descriptor and RPC paths
actually linked into the extension. The local provider-owned DNS protocol is
unchanged.

The migration passed focused unit and compiled-process tests before the local
control copy was deleted, then passed focused unit, binding, vet, build,
compiled-process, and diff checks after deletion. On the designated development
host, the migrated executable code passed full tests, a five-run focused fake-
GoDaddy matrix, full race, 20 normal repeats, 20 shuffled race repeats, all
three native fuzz targets, vet, build, compiled-process integration, two-build
artifact reproducibility, `govulncheck`, `gitleaks`, format, and diff checks.
`govulncheck` found no vulnerabilities and `gitleaks` found no leaks.

Logs, the final binary patch, candidate tree identity, source checksums, and
verification summary are recorded under the parent workspace artifact
`.artifacts/gotth-extension-godaddy-v1-adoption2-20261023`. No live provider,
credential, or DNS state was used. The adoption was checkpointed at
`c860b030fd7ff8297a244c1da1f7d57844bcde5e` on its dedicated branch. Default-
branch movement remains outside that adoption; tag, release, live
provider access, DNS changes, and deployment require separate authorization.

## SRV mutation candidate — 2026-09-20

Source `c1f3525753da982a3ba84a5f443ae8e35dadebc8` passed `make verify`, a
50-run shuffled race repetition, three native fuzz targets, 79.9% package
coverage, and an exact clean-clone `make verify`. Fake GoDaddy tests bind SRV
create/replace/delete to applied, not-applied, unknown, lost-response,
configured-out, stale-state, inverse, and field-validation behavior.

The clean deterministic `1.0.0-alpha.2` candidate archive SHA-256 is
`88ce5fb3c2bd2dc1f66707a8657886055632a31e0a4e7e1cec8e459e0742435f`.
Its configuration metadata contains the exact compiled seven-type mutable
allowlist. No artifact was published and no live provider was contacted.

The independent provider implementation is admitted at source commit
`b313b413dc9ea7d495fa45b21b26c6e3dc94b064`. No GoDaddy credential was read,
no request was sent to GoDaddy, and no live DNS state changed. Release,
deployment, and live-zone acceptance remain separate work.

The private Forgejo repository exists and branch
`workflow/feature/foundation` resolves exactly to the admitted source commit.
The v1.0.0 publication boundary is Forgejo-only. GitHub mirroring is Bryce's
separate handoff and does not block Forgejo tag or release publication.

The exact clean commit passed:

- `make verify`: unit/integration tests, race, vet, build, compiled-process
  Unix-socket authentication, and two-build artifact comparison;
- `go test -count=20 -timeout=120s ./internal/godaddydns`;
- `go test -shuffle=on -count=10 -timeout=120s ./internal/godaddydns`;
- three two-second native fuzz runs for provider JSON, configuration JSON, and
  DNS record canonicalization (4,491, 5,599, and 130 executions in the
  recorded runs);
- a clean-clone `make verify` at the exact source commit;
- a clean-source artifact build with SHA-256
  `03b532d557a9b2894e71ccb224f2d5cd18fcf07f15b94575e1ac48212c9129bd`;
- a production-source boundary scan rejecting subprocess, plugin, ambient
  proxy, default HTTP client, insecure TLS, and legacy GoDaddy credential
  mechanisms, and confirming the two compiled provider hosts occur only in
  the fixed client.

Statement coverage for `internal/godaddydns` is 78.3%. The gap is explicit:
many remaining branches are one-line rejection paths for OS ownership/open
failures, malformed remote pagination permutations, and startup failures that
cannot be induced portably without unsafe host manipulation. The relevant
authority surface is covered: every DNS type, exact snapshots and digests,
stable/unstable pagination, duplicate/unknown JSON, all documented mutation
status classes, timeout, redirect and media-type refusal, create/replace/delete
reconciliation, lost responses, unrelated changes, stale/no-op requests,
capability denial, concurrent mutation serialization, secret-file symlink
refusal, foundation negotiation, and authenticated compiled-process health.

The first cold contract pass corrected two false assumptions before code:
full-zone observation must admit provider-owned and configured-out documented
record types, and GoDaddy record writes are not conditional. The implementation
passes corrected canonical ordering, wildcard observation, operation-ID audit
propagation, strict pagination math, bounded HTTP transport, descriptor-relative
secret opening, and serialized mutation review findings.
