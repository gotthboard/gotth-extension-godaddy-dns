# Contributing

Changes follow the repository's `workflow.toml` sequence: PRD, architecture,
implementation specification, feature scope, implementation, verification,
evidence, and review. Keep this repository limited to the one GoDaddy DNS
records provider. Shared foundation changes belong in `gotth-extensions`;
Stack desired state, approval, journaling, and rollback belong in
`gotth-stack`.

Run `make verify` before proposing admission. Do not add generic invocation,
caller-supplied URLs or headers, registrar operations, nameserver changes,
ambient credentials or proxies, hidden mutation retries, or secret values in
configuration metadata.
