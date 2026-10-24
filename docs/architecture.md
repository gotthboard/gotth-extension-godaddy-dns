# GoDaddy DNS extension architecture

## Boundary

```text
GOTTH Stack controller
  desired DNS state / exact approval / durable journal / rollback policy
                 |
                 | owner-only Unix gRPC + scoped service token
                 v
gotth-extension-godaddy-dns
  ExtensionControl v1
  DNSRecords v1
  strict zone/type allowlist
  before/after snapshot reconciliation
                 |
                 | TLS 1.2+ + Bearer PAT
                 v
https://api.godaddy.com/v3/domains/zones/.../dns-records
```

The host decides what record should exist. The extension does not derive mail
policy, choose hostnames, grant itself zones, or schedule retries. It translates
one admitted action into the fixed GoDaddy record API and reports bounded
evidence.

## Published foundation and control dependencies

The canonical foundation API remains
`github.com/gotthboard/gotth-extensions/pkg/extensions` at `v1.0.0`; this keeps
manifest validation, canonical digests, negotiation, grant, and session
semantics on the published foundation contract. The extension-control wire API
is linked from
`github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1` at `v1.0.0`.
Neither module may be replaced. Source SHA-256, descriptor SHA-256, Go package,
and full RPC paths are pinned by executable checks, and a repository-local
control copy is forbidden.

The DNS record interface is intentionally different: `gotth.dns.provider.v1`
is provider-owned and remains in this repository because the published SDK does
not define it. Moving it would be a separate interface-publication decision,
not cleanup hidden inside foundation adoption.

## Trust and authority

The extension manifest declares `gotth.dns.records` major 1 and capabilities
`dns.records.observe`, `dns.records.create`, `dns.records.replace`, and
`dns.records.delete`. The host grant may select a strict subset. Configuration
further limits exact zones and mutable record types. A mutation request must
satisfy all three:
manifest declaration, host grant, and local configuration.

The GoDaddy PAT is provider-scoped but remains powerful. It is opened from a
private regular file after symlink, ownership, mode, size, and replacement
checks. The value exists only in the outbound Authorization header. Logs and
RPC evidence contain stable categories, correlation IDs, digests, record IDs,
and HTTP status classes—not credentials, response bodies, or arbitrary remote
text.

## State and reconciliation

Observation walks the official paginated list endpoint twice to a fixed
record/page limit, validates every documented GoDaddy record type, sorts
canonical records, and accepts the snapshot only when both walks and their
reported totals are identical. It constructs page numbers locally and never
follows provider-supplied HATEOAS URLs. The snapshot digest is a state
precondition, not authentication. Provider-owned SOA/NS and configured-out
record types are observable state, not mutation authority.

SRV is admitted through the same record methods, not a special endpoint. Its
redundant GoDaddy fields are treated as one canonical owner: `name` must begin
with exactly `service.protocol` and may then contain only a valid relative
owner suffix. This prevents an approval for `_submission._tcp` from carrying
contradictory service/protocol fields. The target is one canonical absolute
name or `.`; priority, weight, and port are explicit bounded integers.

Each mutation follows:

1. observe and require the expected zone snapshot and expected record state;
2. perform exactly one create, replace, or delete request;
3. observe again regardless of the transport result;
4. classify the exact requested delta as `applied`, `not_applied`, or
   `unknown`; and
5. return only canonical before/after digests and bounded inverse material.

The provider performs no automatic mutation retry. Create reconciliation uses
the before snapshot plus the exact canonical record multiplicity; it succeeds
only when one unambiguous new matching record and record ID appear. Replace
and delete use the exact admitted record ID and previous-record digest.

GoDaddy's documented record endpoints provide neither ETags nor conditional
writes. The expected-before checks are therefore not compare-and-swap. The
deployment contract requires the Stack controller path to be the sole writer
for an admitted zone while a mutation is in flight. Reconciliation detects
concurrent divergence after the request and returns `unknown`, but it cannot
undo a record overwrite that occurs inside the read/write race. This limit is
operator-visible and release-blocking unless the sole-writer rule is enforced.

## Rollback model

- create inverse: delete the exact newly observed record ID;
- replace inverse: replace the same record ID with the exact previous record;
- delete inverse: create the exact previous logical record, accepting a new
  provider record ID.

The host separately approves and journals an inverse. If current API state no
longer matches the recorded postcondition, rollback is refused. Public DNS
cache expiry and propagation are observed by the later acceptance layer.

## GoDaddy contract

The implementation contract is pinned to GoDaddy's official Domains v3
OpenAPI document retrieved from
`https://developer.godaddy.com/openapi/domains-v3.json` on 2026-09-20. Later
schema changes require review; they do not silently widen this provider.

Production uses only `https://api.godaddy.com`; OTE uses only
`https://api.ote-godaddy.com`. The API path prefix is fixed to
`/v3/domains`. Authentication is `Authorization: Bearer <PAT>`. The only
allowed resources are:

- `GET/POST /zones/{zone}/dns-records`;
- `PUT/DELETE /zones/{zone}/dns-records/{recordId}`.

Redirects are rejected. TLS 1.2 or newer is required. SOA and NS are rejected
locally even though GoDaddy also returns conflict for managed records.

## Failure and cost model

Validation and canonicalization are linear in bounded input plus `O(n log n)`
sorting for at most 1,000 zone records. One page walk allows at most ten pages
of 100 records and 4 MiB total decoded response data; one admitted observation
requires two identical walks, so it permits at most twenty list requests and
8 MiB. The HTTP transport has bounded connections, headers, idle lifetime, and
response bytes; it may maintain the normal connection goroutines required by
Go's HTTP transport. One mutation costs two admitted observations plus one fixed API request: at most forty list
requests and one mutation, normally four list requests and one mutation for a
single-page zone. That cost is visible and intentional because a paginated,
concurrently changing zone cannot honestly produce an exact precondition from
one walk.
