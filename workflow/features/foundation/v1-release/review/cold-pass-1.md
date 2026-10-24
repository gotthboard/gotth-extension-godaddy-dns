# Cold review pass 1 — production v1.0.0

## Verdict

Accept with constraints.

## Exact flaws found

1. The first release-builder draft produced official `1.0.0` filenames from a
   dirty tree by default. That is not a production release boundary; a caller
   could mistake an uncommitted verification artifact for a publishable object.
2. The first acceptance prose used `idempotency` too broadly. The extension
   has exact-before duplicate suppression after an applied operation, but the
   operation ID is not a provider idempotency key or a persistent replay cache.
3. The prior adoption status still claimed it was waiting for a checkpoint even
   though the exact c860b03 checkpoint and dedicated origin branch already
   existed.

## Required smaller fixes

- Fail closed on dirty release source by default. Permit dirty builds only behind
  an explicit test-only override, record `vcs.modified=true`, and forbid those
  objects from publication. Publish the archive and SBOM as a paired transaction
  with cleanup of only outputs created by the failed attempt.
- Describe the actual replay mechanism: an applied request makes the exact
  before snapshot stale and prevents another write; unchanged or unknown state
  still requires a host journal and explicit retry decision.
- Mark the published-dependency adoption complete at c860b03 while preserving
  the separate default-branch, tag, release, live-DNS, and deployment gates.

All three fixes are present in the reviewed candidate.

## Boundary notes

The patch does not widen provider methods, zones, record types, transport,
credentials, permissions, or runtime configuration. It adds no retry path. The
SPDX tool runs only during release construction and validates the compiled
binary against a fixed module inventory; it is not linked into the provider.
The combined distribution license changes archive bytes intentionally but not
the documented five-file archive shape.

No userspace breakage was found in DNS RPC behavior, trust semantics,
confirmation policy, or local transport. Release tooling intentionally rejects
prerelease names and dirty source that the production v1.0.0 line must not
publish. Contracts and source were checked rather than inferred: the PRD,
implementation specification, generated descriptors, module build information,
Go module sums, and upstream license/notice files agree.

## Remaining constraints

Freeze one exact source candidate, rerun affected artifact and final source
gates, complete a second cold pass against that same product content, and seal
the patch/tree/logs/artifacts externally. Do not claim committed, tagged,
published, deployed, or live-provider accepted state.
