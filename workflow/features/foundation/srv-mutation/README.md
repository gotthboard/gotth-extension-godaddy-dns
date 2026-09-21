# SRV mutation

State: `active`

This child admits SRV through the existing bounded DNS record mechanism so
GOTTH Stack can express `_submission._tcp`. It does not add an endpoint,
capability, credential, retry loop, or generic record escape hatch.

Contracts:

- [PRD](prd.md)
- [architecture](architecture.md)
- [implementation specification](implementation-spec.md)
