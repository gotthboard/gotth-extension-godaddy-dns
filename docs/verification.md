# Verification status

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
The corresponding GitHub API lookup returns 404, so cross-forge parity, tag,
and release publication are explicitly blocked rather than fabricated.

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
