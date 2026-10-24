# Final v1.0.0 defect repair evidence — 2026-10-24

The final review invalidated the prior “no remaining source defect” conclusion.
The repaired source now:

- carries exactly `gotth.dns.records` major 1 minor 0 in the runtime
  binding, rejects any other interface set, enforces it beside capabilities on
  every DNS RPC, and reports the bound list in Handshake;
- admits only 32–256 printable-ASCII service-token bytes;
- separates the published foundation's lexical instance-ID grammar from
  canonical lowercase RFC 4122 version 1–5/variant correlation and operation
  IDs; and
- drives real compiled-binary Handshake, ObserveZone, and CreateRecord RPCs
  through the Unix socket into a fake TLS GoDaddy record endpoint.

Fresh focused, compiled-process, security, source-boundary, reproducibility,
and clean release-build evidence is sealed outside the repository at
`.artifacts/gotth-extension-godaddy-v1-release-final-20261024`. That seal
records the exact commit, tree, toolchain, source manifest, logs, artifact
hashes, and `vcs.modified=false` provenance.

The branch contains an immutable source commit and clean candidate release
files. It is not tagged or published. No remote branch, hosted release,
credential, live provider, DNS record, deployment, Mail, or BB state changed.
