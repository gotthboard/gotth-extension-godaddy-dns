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

The current adoption candidate retains the foundation manifest, grant, session,
and canonicalization API from `github.com/gotthboard/gotth-extensions/pkg/extensions`
at published `v1.0.0`. Extension control messages and gRPC registration now come
from published `github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1` at
`v1.0.0`; the repository-local control schema and generated Go copy are gone.
Both modules are selected without a `replace` directive. The provider-owned
`gotth.dns.provider.v1` interface remains local because it is not part of the
published foundation SDK.

This is an untagged, unpublished candidate ready for checkpoint commit. It
includes the linear foundation and SRV histories, but the default Forgejo
foundation branch has not been moved. Push or default-branch movement is outside
this task; tag, release, credentials, live provider access, live DNS, and
deployment require separate authorization.
