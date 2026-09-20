# Changelog

## Unreleased

### 2026-09-20 — Define the bounded GoDaddy DNS provider

- Froze the provider authority, official v3 record API boundary, file-only PAT
  transport, exact zone/type grants, mutation reconciliation, rollback, and
  verification contract.
- Separated full-schema observation from the smaller mutable type allowlist
  and required two identical bounded pagination walks for exact snapshots.
- Excluded PTR and every registrar, nameserver, forwarding, DNSSEC, domain
  lifecycle, generic HTTP, shell, plugin, and live-DNS path.
