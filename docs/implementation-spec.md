# GoDaddy DNS extension implementation specification

## Runtime files

The executable requires paths, never secret values:

- `GOTTH_EXTENSION_CONFIG_FILE`: private canonical JSON configuration;
- `GOTTH_EXTENSION_BINDING_FILE`: exact instance, manifest, grant, and session
  digests plus the canonical granted-capability subset;
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

The binding's sorted capability list must be a non-empty subset of the
manifest capabilities. Each DNS RPC checks its required capability. The grant
digest is evidence of the host decision; it is not used as a substitute for
the executable capability list.

Configuration schema:

```json
{
  "schema": "gotth.extension.godaddy-dns.config.v1",
  "environment": "production",
  "zones": ["example.com"],
  "record_types": ["A", "AAAA", "CAA", "CNAME", "MX", "TXT"],
  "timeout_seconds": 15
}
```

Zones are lowercase ASCII/punycode names without trailing dots. Record types
are an exact sorted subset of the compiled mutable allowlist: `A`, `AAAA`,
`CAA`, `CNAME`, `MX`, and `TXT`. Observation separately admits every type in
the documented GoDaddy v3 schema: those six plus `NS`, `SOA`, and `SRV`. TTL is
600–86400.

At most 32 zones and six mutable record types are configured. Zone and
zone-relative names are at most 253 and 255 bytes respectively; record data is
at most 512 bytes; record IDs are opaque printable ASCII of 1–256 bytes. The
provider does not trim, truncate, or repair inputs.

## DNS record wire contract

The seam package is `gotth.dns.provider.v1`; service `DNSRecords` exposes:

- `ObserveZone`;
- `CreateRecord`;
- `ReplaceRecord`;
- `DeleteRecord`.

Every request carries a bounded lowercase UUID correlation ID. Mutations also
carry a distinct host operation ID and expected-before snapshot digest.
Replace/delete carry record ID and expected record digest. Records contain
only zone-relative name, enum type, canonical data, TTL, and the fields valid
for that type: `priority` for MX; `service`, `protocol`, `priority`, `weight`,
and `port` for SRV; and `flag` and `tag` for CAA. Arbitrary provider JSON is
impossible. SRV, NS, and SOA can appear only in observation responses and
snapshot preconditions; they are rejected from mutation inputs.

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

Names use `@` for apex or lowercase relative labels; a wildcard is admitted
only as the complete leftmost label. CNAME/MX targets are
lowercase absolute names with one trailing dot. TXT data remains exact UTF-8
after control/NUL and size rejection. IP addresses use canonical textual form.

## HTTP client

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
with the exact postcondition may still be `applied`. Mutation RPC handlers do
not retry. The HTTP API has no conditional-write primitive; this algorithm
does not claim atomic compare-and-swap. Runtime admission requires the Stack
controller path to be the sole external writer during each zone mutation. The
service also serializes its complete precondition/write/reconciliation
sequence so two admitted RPCs cannot write concurrently.

## Artifact and source boundary

The deterministic archive is
`gotth-extension-godaddy-dns-<version>-linux-amd64.tar.gz` and contains only
the static executable, `manifest.json`, `configuration-metadata.json`,
`LICENSE`, and `SHA256SUMS`.

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
