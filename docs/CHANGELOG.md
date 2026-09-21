# Changelog

## Unreleased

### 2026-09-20 20:10 CDT — Define bounded SRV mutation

- Added the child PRD, architecture, implementation specification, workflow
  state, and verification contract for SRV create, replace, and delete through
  the existing exact-record boundary.
- Pinned the change to the official Domains v3 `DNSRecord` schema and required
  coherent owner/service/protocol fields, all numeric SRV fields, canonical
  absolute or `.` targets, existing reconciliation semantics, configuration
  metadata parity, and a reproducible `1.0.0-alpha.2` candidate artifact.
- NS/SOA mutation, live credentials, live DNS, publication, tags, and releases
  remain excluded.

### 2026-09-20 — Define the bounded GoDaddy DNS provider

- Froze the provider authority, official v3 record API boundary, file-only PAT
  transport, exact zone/type grants, mutation reconciliation, rollback, and
  verification contract.
- Separated full-schema observation from the smaller mutable type allowlist
  and required two identical bounded pagination walks for exact snapshots.
- Excluded PTR and every registrar, nameserver, forwarding, DNSSEC, domain
  lifecycle, generic HTTP, shell, plugin, and live-DNS path.
- Implemented and admitted the fixed GoDaddy v3 DNS record client, DNSRecords
  gRPC seam, authenticated process runtime, exact reconciliation, deterministic
  artifact, and fake-provider conformance at `b313b413dc9e`.
