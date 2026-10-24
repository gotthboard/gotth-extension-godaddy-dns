# SRV mutation

State: `done`

This child admits SRV through the existing bounded DNS record mechanism so
GOTTH Stack can express `_submission._tcp`. It does not add an endpoint,
capability, credential, retry loop, or generic record escape hatch.

Contracts:

- [PRD](prd.md)
- [architecture](architecture.md)
- [implementation specification](implementation-spec.md)

Exact source, artifact, verification, limits, and cold review records are under
`evidence/` and `review/`. This completes the source child only. The production parent remains gated on
an immutable commit and separately authorized Forgejo tag/publication. GitHub
mirroring is a separate handoff, not a Forgejo release blocker. Live credentials,
DNS mutation, deployment, and propagation proof remain separate operations.
