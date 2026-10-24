# Changelog

## v1.0.0 — 2026-10-23

- Fixed the production runtime, manifest, archive, and release documentation to
  version `1.0.0`; prerelease versions are no longer accepted by the runtime
  or artifact builder.
- Added fake-GoDaddy create fixtures for A, AAAA, CAA, CNAME, MX, TXT, and SRV,
  post-application duplicate rejection, explicit rollback-material assertions,
  pagination inconsistency checks, TLS failure classification, duplicate local
  authentication rejection, and redacted compiled-process startup failure.
- Added an exact linked-module inventory, pinned upstream license and notice
  verification, a combined distribution license, embedded Git build identity,
  and a deterministic SPDX 2.3 sidecar bound to the archive checksum.
- Added release, verification, upgrade, rollback, sole-writer, and deferred-scope
  documentation. No tag, hosted release, credential, live provider call, DNS
  mutation, or deployment was performed.

### 2026-10-23 — Adopt published Extensions V1 foundation and SDK

- Selected published `gotth-extensions v1.0.0` and `gotth-sdk v1.0.0` without
  replacements, while preserving foundation imports from `pkg/extensions`.
- Migrated extension-control imports and server/client registration to the SDK,
  added exact source/descriptor/RPC binding gates, and removed the tested
  repository-local control schema and generated Go copies. The provider-owned
  DNS interface remains local.
- Based the candidate on the SRV branch tip, which is a linear descendant of
  the foundation tip, so both histories are retained without merge, rebase, or
  overwrite. The default foundation branch was not moved because commit and
  push are outside this task.
- No credential was read, no live provider was contacted, and no DNS, tag,
  release, or deployment state changed.

### 2026-09-20 20:20 CDT — Admit bounded SRV mutation source

- Added SRV to the compiled and metadata mutation allowlists without adding a
  capability, endpoint, retry path, or generic JSON mechanism.
- Enforced coherent SRV owner/service/protocol fields, required numeric
  fields, canonical absolute or `.` target, exact configured-type authority,
  and the existing serialized exact-before/reconciliation/inverse boundary.
- Bound fake-provider create/replace/delete and outcome matrices, compiled
  process startup, deterministic `1.0.0-alpha.2` artifact, clean clone, race,
  repeat, shuffle, fuzz, coverage, and two clean final reviews to source
  `c1f3525753da982a3ba84a5f443ae8e35dadebc8`.
- Public GitHub mirroring, tag/artifact publication, live credentials, live DNS
  mutation, and propagation acceptance remain blocked.

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
