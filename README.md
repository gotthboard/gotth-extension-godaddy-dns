# GOTTH GoDaddy DNS Extension

`gotth-extension-godaddy-dns` is an out-of-process, least-privilege DNS record
provider for GoDaddy-managed zones. It implements the GOTTH extension control
contract and one DNS-record interface. It does not manage domain registration,
nameservers, forwarding, contacts, privacy, renewal, or reverse DNS.

The host owns desired state, approval, journaling, retry, rollback, audit,
credential storage, supervision, and authoritative-DNS propagation checks.
The extension owns only strict request validation, GoDaddy API translation,
bounded observation, and mutation outcome reconciliation.

This repository defines the bounded v1.0.0 source and release contract. See
`docs/prd.md`, `docs/architecture.md`, `docs/implementation-spec.md`, and
`docs/release.md`. A source or release build does not itself authorize live
credential use, DNS mutation, or deployment.

The v1.0.0 release retains the foundation manifest, grant, session,
and canonicalization API from `github.com/gotthboard/gotth-extensions/pkg/extensions`
at published `v1.0.0`. Extension control messages and gRPC registration now come
from published `github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1` at
`v1.0.0`; the repository-local control schema and generated Go copy are gone.
Both modules are selected without a `replace` directive. The provider-owned
`gotth.dns.provider.v1` interface remains local because it is not part of the
published foundation SDK.

The production release fixes the runtime and artifact version to `1.0.0`,
adds deterministic SPDX 2.3 release metadata and linked dependency license
coverage, and binds fake-provider fixtures for every mutable record shape. The
archive embeds Git build identity and remains reproducible. Commit, tag, Forgejo publication, credential use, live provider access, DNS
mutation, and deployment are explicit operator actions rather than build side
effects.
