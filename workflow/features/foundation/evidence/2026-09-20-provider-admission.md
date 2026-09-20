# GoDaddy DNS provider admission evidence

## Scope

Exact source commit:
`b313b413dc9ea7d495fa45b21b26c6e3dc94b064`.

This evidence admits the independent source implementation and deterministic
artifact mechanism. It does not claim a public release, deployed process,
valid live PAT, exclusive live-zone authority, or successful public DNS
propagation.

Private Forgejo branch `workflow/feature/foundation` resolves exactly to the
candidate. The public GitHub repository is absent (API 404), which blocks
cross-forge tag and artifact publication.

## Verified mechanism

- Fixed production/OTE Domains v3 hosts and four DNS-record methods only.
- Full-schema observation separated from the smaller mutable type allowlist.
- Two identical bounded page walks per admitted snapshot.
- Exact expected snapshot, record digest, logical multiplicity, and distinct
  operation identifier before one mutation request.
- Serialized mutation sequences and no mutation retry.
- Applied/not-applied/unknown reconciliation including a lost HTTP response.
- Inverse material preserves logical state without claiming deleted IDs return.
- Owner-only files/socket, descriptor-relative PAT open with `O_NOFOLLOW`,
  service-token authentication, and exact granted-capability enforcement.
- No configurable URL/header/method, ambient proxy, redirect, compression,
  subprocess, plugin, registrar, nameserver, or domain-lifecycle path.

## Commands and results

- `make verify`: PASS.
- `go test -count=20 -timeout=120s ./internal/godaddydns`: PASS.
- `go test -shuffle=on -count=10 -timeout=120s ./internal/godaddydns`: PASS.
- `go test -coverprofile=coverage.out ./internal/godaddydns`: PASS, 78.3%.
- Each native fuzz target for provider JSON, configuration, and record
  canonicalization ran for two seconds: PASS.
- `GOTTH_EXTENSION_REQUIRE_CLEAN=1 ./scripts/build-artifact.sh
  1.0.0-alpha.1 <new-output>`: PASS.
- Clean clone at the exact commit, `make verify`: PASS.
- Production-source boundary scan: PASS.

Artifact SHA-256:
`03b532d557a9b2894e71ccb224f2d5cd18fcf07f15b94575e1ac48212c9129bd`.

## Explicit limits

GoDaddy provides no conditional DNS-record mutation primitive. The provider
serializes its own mutations, but deployment still requires the Stack path to
be the sole external zone writer during each operation. Public authoritative
DNS propagation remains a later acceptance boundary. The 78.3% coverage gap is
recorded in `docs/verification.md`; it is not being misrepresented as 100%.
