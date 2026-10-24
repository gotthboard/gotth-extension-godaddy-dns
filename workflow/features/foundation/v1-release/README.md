# Production v1.0.0 release preparation

State: `blocked` pending an immutable candidate commit and separately
authorized tag/publication. Source verification may complete before those
actions without fabricating release state.

Base checkpoint:
`c860b030fd7ff8297a244c1da1f7d57844bcde5e`.

## Accepted product slice

- Runtime, manifest, archive, and user-visible version are exactly `1.0.0`.
- Foundation and control dependencies remain exact published v1.0.0 modules,
  without replacements or a repository-local control copy.
- Full-zone stable observation and bounded create/replace/delete behavior remain
  restricted to fixed GoDaddy v3 record endpoints.
- Fake-provider fixtures cover A, AAAA, CAA, CNAME, MX, TXT, and SRV plus
  pagination, status, transport, lost-response, reconciliation, post-application
  duplicate suppression, concurrency classification, and inverse material.
- Compiled-process tests cover owner-only Unix transport, token authentication,
  duplicate authentication rejection, handshake/health, and generic redacted
  startup failure.
- The release archive carries the static executable, canonical manifest,
  configuration metadata, combined project/dependency license text, and inner
  checksums. Deterministic SPDX 2.3 and provenance sidecars plus portable outer
  checksums bind the clean commit/tree, exact toolchain and `GOEXPERIMENT`, build
  parameters, archive, linked modules, licenses, and SBOM.
- Release, upgrade, rollback, sole-writer, and verification instructions are
  explicit.

## Required closure

- [x] local focused tests and source-boundary checks;
- [x] development-host full, race, repeat, shuffle, fuzz, process, vet, build,
      reproducibility, vulnerability, secret, format, and diff gates;
- [x] deterministic verification archive/SBOM/provenance/checksums copied into the authorized
      evidence directory and explicitly marked non-publishable while dirty;
- [x] sealed source patch, candidate tree, source manifest, raw logs, artifact
      checksums, and final evidence root;
- [x] two fresh cold remediation reviews against the final sealed candidate;
- [ ] immutable source commit (not authorized in this writer task);
- [ ] tag and hosted release (not authorized in this writer task);
- [ ] production credential, live DNS acceptance, and deployment (explicitly
      outside this writer task).

Registrar, DNSSEC, nameserver, forwarding, PTR, ACME, generic HTTP, batch
mutation, propagation policy, and other roadmap work remain deferred rather
than being smuggled into v1.0.0.
