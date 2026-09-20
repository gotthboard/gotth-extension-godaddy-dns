# GOTTH GoDaddy DNS Extension

`gotth-extension-godaddy-dns` is an out-of-process, least-privilege DNS record
provider for GoDaddy-managed zones. It implements the GOTTH extension control
contract and one DNS-record interface. It does not manage domain registration,
nameservers, forwarding, contacts, privacy, renewal, or reverse DNS.

The host owns desired state, approval, journaling, retry, rollback, audit,
credential storage, supervision, and authoritative-DNS propagation checks.
The extension owns only strict request validation, GoDaddy API translation,
bounded observation, and mutation outcome reconciliation.

This repository is unreleased. See `docs/prd.md`, `docs/architecture.md`, and
`docs/implementation-spec.md`.
