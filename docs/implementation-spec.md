# GoDaddy DNS extension implementation specification

## Runtime files

The executable requires paths, never secret values:

- `GOTTH_EXTENSION_CONFIG_FILE`: private canonical JSON configuration;
- `GOTTH_EXTENSION_BINDING_FILE`: exact instance, manifest, grant, and session
  digests plus the exact negotiated interface and canonical granted-capability
  subset;
- `GOTTH_EXTENSION_SERVICE_TOKEN_FILE`: local RPC bearer token;
- `GOTTH_EXTENSION_SECRET_DIR/godaddy.pat`: GoDaddy Bearer PAT;
- `GOTTH_EXTENSION_SOCKET`: absent socket below a private runtime directory.

Files must be regular, non-symlinked, owned by the effective UID, and have no
group/other permission bits (normally `0600` or `0400`). Directories must be
owned by the effective UID and private. The secret directory is held open and
the PAT is opened relative to that descriptor with `O_NOFOLLOW`; other runtime
directories are checked by `lstat` plus `open`/`fstat` before use.
Configuration is at most 64 KiB, token/PAT files at most 8 KiB, and unknown or
duplicate JSON names, trailing data, controls, and noncanonical values reject.
The local service token is 32–256 printable ASCII bytes (`0x21` through
`0x7e`); whitespace, DEL, non-ASCII UTF-8, and length violations reject
before the socket is created.

The binding must contain exactly the negotiated
`gotth.dns.records` interface at major 1, minor 0. The binding's sorted
capability list must be a non-empty subset of the manifest capabilities. Each
DNS RPC checks both that exact interface and its required capability. Handshake
echoes the bound interface list rather than reconstructing an unbound static
claim. The grant digest is evidence of the host decision; it is not used as a
substitute for the executable interface and capability lists.

Configuration schema:

```json
{
  "schema": "gotth.extension.godaddy-dns.config.v1",
  "environment": "production",
  "zones": ["example.com"],
  "record_types": ["A", "AAAA", "CAA", "CNAME", "MX", "SRV", "TXT"],
  "timeout_seconds": 15
}
```

Zones are lowercase ASCII/punycode names without trailing dots. Record types
are an exact sorted subset of the compiled mutable allowlist: `A`, `AAAA`,
`CAA`, `CNAME`, `MX`, `SRV`, and `TXT`. Observation admits every type in the
documented GoDaddy v3 schema; `NS` and `SOA` remain observation-only. TTL is
600–86400.

At most 32 zones and seven mutable record types are configured. Zone and
zone-relative names are at most 253 and 255 bytes respectively; record data is
at most 512 bytes; record IDs are opaque printable ASCII of 1–256 bytes. The
provider does not trim, truncate, or repair inputs.

## DNS record wire contract

The seam package is `gotth.dns.provider.v1`; service `DNSRecords` exposes:

- `ObserveZone`;
- `CreateRecord`;
- `ReplaceRecord`;
- `DeleteRecord`.

Every request carries a canonical lowercase RFC 4122 UUID correlation ID with
version 1–5 and the RFC 4122 variant bits. Mutations also carry a distinct host
operation ID with the same UUID semantics and an expected-before snapshot
digest. The foundation instance ID remains governed by the published V1
lexical `8-4-4-4-12` lowercase-hex grammar, which deliberately does not
constrain version or variant bits; request IDs do not reuse that looser helper.
Replace/delete carry record ID and expected record digest. Records contain
only zone-relative name, enum type, canonical data, TTL, and the fields valid
for that type: `priority` for MX; `service`, `protocol`, `priority`, `weight`,
and `port` for SRV; and `flag` and `tag` for CAA. Arbitrary provider JSON is
impossible. For SRV, `service` is one lowercase underscore-prefixed label,
`protocol` is `_tcp`, `_udp`, or `_sctp`, and `name` must equal
`service.protocol` or begin with `service.protocol.` followed by a valid
relative owner suffix. `data` is a lowercase absolute target with one trailing
dot or the exact unavailable-service target `.`. `priority`, `weight`, and
`port` are all required and range from 0 through 65535. NS and SOA can appear
only in observation responses and snapshot preconditions.

Responses contain stable enum outcome/reason, before/after snapshot digests,
canonical record identity, and inverse material. No response carries remote
body/header text, PAT, service token, URLs, or arbitrary metadata.

Create requires the desired logical-record multiplicity from the admitted
before snapshot. Replace requires the exact provider record ID and prior
logical digest and rejects a desired record equal to the prior logical record.
Delete requires the exact provider record ID and prior logical digest. These
checks reject stale, ambiguous, and no-op mutation requests before HTTP I/O.

## Canonical state

Canonical records sort by `(type, name, data, priority, service, protocol,
weight, port, flag, tag, ttl, record_id)`. Snapshot SHA-256 covers schema,
exact zone, and the sorted records.
Record digest excludes record ID for logical equality; provider identity is a
separate bounded field. Duplicate logical records remain distinct by record ID.

CAA property tags follow RFC 8659: one through 255 ASCII alphanumeric
characters. Matching is case-insensitive and canonical state stores lowercase;
valid extension tags beyond `issue`, `issuewild`, and `iodef` remain observable
and mutable. Observations preserve any 0–255 flag byte for forward compatibility;
mutations admit only 0 or the issuer-critical bit 128 and reject reserved bits. Names use `@` for apex or lowercase relative labels; a wildcard is
admitted only as the complete leftmost label. CNAME/MX targets are
lowercase absolute names with one trailing dot. TXT data remains exact UTF-8
after control/NUL and size rejection. IP addresses use canonical textual form.

## HTTP client

With `totalRequired=true`, non-empty responses require consistent positive
`totalItems` and `totalPages`. The official empty response shape omits both
totals and carries empty `items` and `links`; it is admitted as a completed
zero-record walk.

The production base URL is compiled, not configured. The client disables
redirects, compression, and ambient proxies; bounds response headers,
connections, and idle lifetime; requires TLS 1.2+; sets one bounded user agent,
`Authorization: Bearer`, `Accept: application/json`, `Content-Type` for
writes, and `X-Request-Id` from the correlation ID for reads and the distinct
host operation ID for the one mutation. Each call requires a deadline and uses
the earlier caller/configured timeout.

Each full-zone walk uses locally constructed `page=1..10`, `pageSize=100`, and
`totalRequired=true`; provider-returned links are never followed. Totals and
page contents must be internally consistent, and two consecutive walks must
produce byte-identical canonical projections before a snapshot is returned.

JSON success responses must declare `application/json`; parameters are
accepted but other media types reject. Response bodies are bounded before
strict decoding. Only documented 200, 201, and 204 success statuses are
admitted per method. 400/401/403/404/409/422/429
map to fixed reason codes. Other status, decode, TLS, DNS, timeout, and transport
failures never include remote text.

## Mutation classification

- `applied`: the after snapshot contains exactly the requested postcondition
  and the rest of the before snapshot changed only as permitted by that one
  action;
- `not_applied`: the after snapshot equals the exact before snapshot;
- `unknown`: every other state, including unrelated concurrent change.

An HTTP success with an unexpected after snapshot is `unknown`. An HTTP error
with the exact postcondition may still be `applied`. The client applies the configured timeout end to end across both complete
walks, not independently to each page. The write runs under a shorter child
deadline that reserves that full observation timeout plus an explicit 50 ms
handoff margin. Insufficient remaining time rejects before mutation. Mutation RPC
handlers do not retry. The HTTP API has no conditional-write primitive; this algorithm
does not claim atomic compare-and-swap. Runtime admission requires the Stack
controller path to be the sole external writer during each zone mutation. The
service also serializes its complete precondition/write/reconciliation
sequence so two admitted RPCs cannot write concurrently.

## Published dependency boundary

`go.mod` must select exactly `github.com/gotthboard/gotth-extensions v1.0.0`
and `github.com/gotthboard/gotth-sdk v1.0.0` with no `replace` directive.
Control implementation and tests import the SDK generated package. Verification
checks the published foundation and SDK control sources against the same pinned
SHA-256, checks the SDK source binding and foundation descriptor digest, hashes
the descriptor linked into this program, pins both control RPC paths, and fails
if any local `proto/gotth/extensions/v1/control*` copy or import reappears.

The local `proto/gotth/dns/provider/v1` source and generated Go remain the
provider-owned DNS seam and are not deleted by this adoption.

## Artifact and source boundary

The deterministic archive is
`gotth-extension-godaddy-dns-1.0.0-linux-amd64.tar.gz` and contains only
the static executable, `manifest.json`, `configuration-metadata.json`,
`LICENSE`, and `SHA256SUMS`. `LICENSE` combines the project license with the
license and notice text required by the exact linked dependency inventory. A deterministic SPDX 2.3 sidecar binds the archive checksum, embedded Git
source identity, exact linked modules, module sums, and declared licenses. A
deterministic provenance sidecar binds archive and SBOM hashes to the clean
commit/tree, `v1.0.0`, exact Go executable identity and `GOEXPERIMENT`, target,
build flags, and source epoch. A basename-only outer `SHA256SUMS` covers all
three release objects without changing the runtime archive contents. Publication
requires a clean source tree and `vcs.modified=false` in the binary build
information and provenance.

Production source may use only fixed `net/http` requests from the provider
client. It may not import plugin machinery, execute subprocesses, invoke a
shell, accept arbitrary URLs/headers/methods, use environment proxy settings,
or implement any non-record GoDaddy endpoint.

## Verification

- strict configuration, file, descriptor, secret, and redaction matrices;
- foundation manifest/grant/session vectors pinned to the admitted commit;
- protobuf descriptor scan for generic bytes/maps/secret values;
- fake TLS GoDaddy server proving exact methods, paths, headers, bodies,
  pagination, two-walk stability, all observed record types, mutation-type
  rejection, status mapping, limits, redirects, and no proxy use;
- create/replace/delete applied/not-applied/unknown matrices including lost
  responses and concurrent changes;
- authenticated compiled-process Unix-socket handshake/health plus fake-TLS
  DNS RPC integration without a live provider;
- race, repeat, shuffle, coverage, fuzz, build, deterministic artifact,
  clean-copy, and two cold reviews;
- no live GoDaddy credential or zone mutation in automated verification.
