# GoDaddy DNS extension PRD

## Problem

GOTTH Stack requires one independently admitted DNS provider before Mail
identity, certificate, and public-host composition can proceed. The current
GoDaddy zone must not be manipulated by generic HTTP, arbitrary record, or
ambient credential machinery.

## Product boundary

The extension observes and mutates an exact allowlist of DNS resource records
in exact GoDaddy-managed zones. The host supplies already-approved operations
over an authenticated owner-only local transport. GoDaddy remains the API
authority. Public authoritative DNS is a separate observation boundary.

The provider uses the official Domains v3 records API. Record mutations are
synchronous at the API boundary but may still have ambiguous transport
outcomes. The extension never retries a mutation blindly.

## Requirements

- `GDDNS-001`: Run as a separate unprivileged process over an owner-only Unix
  socket and implement GOTTH extension control v1.
- `GDDNS-002`: Load one GoDaddy Bearer PAT from an owner-only regular file.
  Secret values never enter arguments, environment values, RPC payloads,
  manifests, logs, errors, health, or artifacts.
- `GDDNS-003`: Accept only a fixed environment (`production` or `ote`), an
  exact non-empty zone allowlist, an exact record-type allowlist, and bounded
  timeouts. Runtime configuration contains no arbitrary URL or header.
- `GDDNS-004`: Expose read-only full-zone observation plus one-record create,
  replace, and delete methods. Observation accepts every record type in the
  documented GoDaddy v3 record schema; mutation accepts only the configured
  subset of the compiled mutable allowlist. Reject SOA/NS mutation, wildcard
  zone selection, cross-zone record IDs, and every non-record GoDaddy
  operation.
- `GDDNS-005`: Require exact expected-before snapshot and record digests plus a
  host-generated operation ID for every mutation. Observe after every call and
  return `applied`, `not_applied`, or `unknown`; never fabricate success.
- `GDDNS-005A`: State the provider's concurrency limit honestly: GoDaddy's v3
  record mutation endpoints expose no ETag or conditional write. Deployment
  therefore requires this controller path to be the sole zone writer while a
  mutation is in flight. Post-write reconciliation detects divergence but
  cannot prevent an external writer from being overwritten in the read/write
  race.
- `GDDNS-006`: Bound pagination, records, names, data, TTL, request/response
  bytes, deadlines, and error categories. A snapshot is admitted only after
  two consecutive bounded page walks produce identical canonical state and
  totals. Discard remote error bodies and never reflect hostile provider text.
- `GDDNS-007`: Preserve enough previous record state for the host to authorize
  an honest inverse. Delete rollback recreates logical record state and does
  not pretend GoDaddy record IDs survive deletion.
- `GDDNS-008`: Publish deterministic Linux/amd64 artifacts containing the
  executable, canonical manifest, configuration metadata, license, and
  checksums with exact source/version identity.
- `GDDNS-009`: Pass foundation negotiation, authenticated process integration,
  fake-GoDaddy outcome matrices, race/repeat/shuffle, artifact reproducibility,
  source-boundary review, and two cold admission passes.
- `GDDNS-010`: Admit SRV as a configured mutable type using only the official
  Domains v3 `DNSRecord` fields. Require coherent owner `name`, `service`, and
  `protocol`, all three integer fields (`priority`, `weight`, and `port`), an
  absolute canonical target or the RFC-defined `.` unavailable-service target,
  and the same exact-before/reconciliation/inverse rules as every other
  mutable record. NS and SOA remain observation-only.
- `GDDNS-011`: Select published `gotth-extensions v1.0.0` and `gotth-sdk
  v1.0.0` without module replacement. Preserve the foundation package import
  for manifest, grant, session, and canonicalization behavior; consume the
  extension-control protobuf and gRPC API from the SDK; and reject a duplicate
  repository-local extension-control source or generated copy. The separate
  provider-owned DNS interface remains local.

## Non-goals

- Live DNS mutation during development or conformance.
- PTR/rDNS, registrar operations, nameserver delegation, DNSSEC, forwarding,
  contacts, privacy, renewal, transfer, or domain purchase.
- ACME client behavior or certificate storage.
- Generic REST proxying, caller-supplied endpoints, arbitrary headers, batch
  scripts, shell execution, plugin loading, or in-process provider code.
- DNS propagation claims based only on GoDaddy API state.

## Acceptance

All request validation, exact record canonicalization, pagination, API status,
timeout, cancellation, ambiguous write, reconciliation, inverse material,
secret redaction, transport authentication, and artifact behavior has tests.
No live zone, credential, tag, release, or deployment is used as a test
fixture.
