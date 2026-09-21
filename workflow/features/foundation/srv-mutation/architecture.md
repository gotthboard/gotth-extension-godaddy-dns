# Architecture: bounded SRV mutation

SRV uses the existing `DNSRecords` RPC and fixed GoDaddy v3 record endpoints.
There is no SRV-specific transport or authority. Admission remains the
intersection of manifest capability, host grant, exact configured zone, and
configured record type.

Canonicalization treats GoDaddy's redundant SRV fields as a single owner.
`name` must be `service.protocol` or `service.protocol.<relative-suffix>`.
The service is one underscore-prefixed lowercase label and protocol is one of
the three compiled values. Target, priority, weight, port, TTL, and auxiliary
field rules are validated before HTTP I/O.

The existing serialized precondition/write/reconciliation sequence and
applied/not-applied/unknown classifier remain unchanged. This is an additional
record shape, not an additional mutation mechanism.
