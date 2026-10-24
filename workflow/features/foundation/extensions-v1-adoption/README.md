# Published Extensions V1 adoption

State: `done` at `c860b030fd7ff8297a244c1da1f7d57844bcde5e`

This child moves the existing foundation and SRV candidate to the published
`gotth-extensions v1.0.0` and `gotth-sdk v1.0.0` contracts without replacement.
Foundation package imports remain intact; only extension-control protobuf/gRPC
imports move to the SDK. The duplicate local control source/generated copies are
removed after pre-deletion tests. The provider-owned DNS protocol remains local.

The branch starts at SRV tip `8529e8d13dcd7634c6810b1b937fb652564283a4`.
That commit descends linearly from foundation tip
`ed2707ac504bb5a3f2aba7dd6345a2ea93008270`, so the candidate retains both
histories without merge, rebase, force update, or overwrite. The adoption was checkpointed at the exact commit above and the dedicated
`origin/workflow/feature/extensions-v1-adoption` branch resolves to it. The
default `origin/workflow/feature/foundation` branch remains unchanged; moving a
default branch is a separate root-release authority decision, not an adoption
implementation blocker.

No live credentials, provider calls, DNS changes, tags, releases, or deployment
are in scope. Evidence is under `evidence/` and the parent workspace artifact.
