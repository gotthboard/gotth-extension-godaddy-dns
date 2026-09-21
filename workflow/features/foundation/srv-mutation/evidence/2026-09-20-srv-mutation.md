# SRV mutation verification — 2026-09-20

## Exact candidate

- contract: `fc759b22bb3c5f3c73b2fb51fb95440f61e4373b`;
- implementation: `5700bec0c5d86dd4fc8aa6567a423097c3289c43`;
- reconciliation evidence: `c1f3525753da982a3ba84a5f443ae8e35dadebc8`.

## Contract evidence

The official GoDaddy Domains v3 OpenAPI `DNSRecord` schema was read on
2026-09-20. It admits SRV in the record type enum and defines `name`, `data`,
`ttl`, `priority`, `service`, `protocol`, `weight`, and `port`; only NS and SOA
are documented as read-only by the fixed replace/delete endpoints.

The implementation adds SRV only to the existing mutable type intersection.
It rejects contradictory owner/service/protocol fields, malformed service or
protocol labels, missing or out-of-range numeric fields, wildcard suffixes,
malformed targets, auxiliary fields, and configured-out SRV before HTTP I/O.
Create, replace, and delete retain exact snapshots, record digests,
multiplicity, one-write behavior, post-write reconciliation, inverse material,
and the documented sole-writer limitation.

## Verification

- `make verify` — full normal, race, vet, build, compiled-process, deterministic
  artifact, checksums, and diff checks passed;
- `go test -race -count=50 -shuffle=on ./internal/godaddydns` — passed;
- fuzz targets for strict provider JSON, configuration, and record
  canonicalization ran for three seconds each — passed;
- package coverage: 79.9%, up from 78.3%. The remaining gap is defensive
  filesystem, socket, HTTP transport, and rare failure handling; material SRV
  authorization, validation, applied/not-applied/unknown, lost-response,
  concurrency, stale-state, and process behavior is covered;
- exact clean clone at `c1f3525`, `make verify` — passed;
- source-boundary behavior remains fixed record HTTP only, with no shell,
  subprocess, generic endpoint, ambient proxy, redirect, or live provider;
- two fresh final cold passes — clean.

## Artifact

- candidate: `gotth-extension-godaddy-dns-1.0.0-alpha.2-linux-amd64.tar.gz`;
- SHA-256: `88ce5fb3c2bd2dc1f66707a8657886055632a31e0a4e7e1cec8e459e0742435f`;
- retained on the development evidence host under the versioned candidate
  artifact directory;
- not tagged, published, mirrored publicly, or deployed.

## Limits

The private source branch and candidate artifact are admissible for Stack
composition tests. The parent release remains blocked on a public GitHub
repository/mirror, cross-forge parity, immutable tag/artifact publication,
live GoDaddy credentials, sole-writer enforcement, live-zone mutation,
authoritative DNS observation, and propagation acceptance.
