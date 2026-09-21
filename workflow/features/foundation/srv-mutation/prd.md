# PRD: bounded SRV mutation

## Problem

GOTTH Mail readiness requires an exact `_submission._tcp` SRV record. The
provider already observes SRV but its compiled mutation allowlist rejects it,
so Stack cannot honestly compose required DNS state.

## Requirements

- Add only SRV to the compiled/configured mutable record types.
- Use the official GoDaddy Domains v3 `DNSRecord` fields and existing record
  create, replace, and delete endpoints.
- Reject incoherent `name`, `service`, or `protocol`, omitted numeric fields,
  out-of-range values, malformed targets, auxiliary CAA fields, and every
  unconfigured type or zone.
- Preserve exact-before snapshots, record digests, multiplicity checks,
  single-write behavior, post-write reconciliation, inverse material, secret
  redaction, and sole-writer limitations.
- Update configuration metadata and the deterministic candidate artifact.

## Non-goals

Live DNS mutation, credentials, propagation claims, NS/SOA writes, new RPCs,
generic record JSON, PTR/rDNS, ACME behavior, publication, and release tags.
