# Implementation specification: bounded SRV mutation

1. Add `SRV` to the sorted compiled mutable map and configuration metadata.
2. In `canonicalRecord`, admit SRV for mutation and observation only when:
   - the service is one lowercase `_`-prefixed label of at most 63 bytes;
   - protocol is `_tcp`, `_udp`, or `_sctp`;
   - name is exactly `service.protocol` or that prefix plus a valid relative
     suffix;
   - target is `.` or a lowercase absolute name with one trailing dot;
   - priority, weight, and port are non-nil and at most 65535;
   - CAA fields are absent.
3. Preserve the existing protobuf projection and strict GoDaddy JSON body.
4. Extend record, config, service, fake HTTP, and compiled-process tests across
   create/replace/delete; applied/not-applied/unknown; lost response; stale
   snapshot/digest; malformed field combinations; and secret redaction.
5. Run full, race, repeat, shuffle, fuzz, coverage, clean-clone, source-boundary,
   process, and reproducible `1.0.0-alpha.2` artifact gates, then two clean cold
   reviews.
