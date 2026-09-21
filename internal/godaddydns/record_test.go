package godaddydns

import (
	"testing"
)

func u32(v uint32) *uint32 { return &v }

func TestCanonicalRecordTypes(t *testing.T) {
	t.Parallel()
	cases := []apiRecord{
		{RecordID: "a1", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600},
		{RecordID: "a2", Name: "mail", Type: "AAAA", Data: "2001:0db8::1", TTL: 3600},
		{RecordID: "a3", Name: "alias", Type: "CNAME", Data: "mail.example.com.", TTL: 3600},
		{RecordID: "a4", Name: "@", Type: "MX", Data: "mail.example.com.", TTL: 3600, Priority: u32(10)},
		{RecordID: "a5", Name: "_dmarc", Type: "TXT", Data: "v=DMARC1; p=reject", TTL: 3600},
		{RecordID: "a6", Name: "@", Type: "NS", Data: "ns1.example.com.", TTL: 3600},
		{RecordID: "a7", Name: "_submission._tcp", Type: "SRV", Data: "mail.example.com.", TTL: 3600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"},
		{RecordID: "a8", Name: "@", Type: "SOA", Data: "ns1.example.com. hostmaster.example.com. 1 3600 600 604800 600", TTL: 3600},
		{RecordID: "a9", Name: "@", Type: "CAA", Data: "letsencrypt.org", TTL: 3600, Flag: u32(0), Tag: "issue"},
	}
	for _, tc := range cases {
		got, err := canonicalRecord(tc, true)
		if err != nil {
			t.Fatalf("%s: %v", tc.Type, err)
		}
		if got.Type == "AAAA" && got.Data != "2001:db8::1" {
			t.Fatalf("IPv6 was not canonicalized: %q", got.Data)
		}
	}
}

func TestMutationTypeAndFieldRejection(t *testing.T) {
	t.Parallel()
	cases := []apiRecord{
		{RecordID: "provider", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600},
		{Name: "@", Type: "NS", Data: "ns1.example.com.", TTL: 600},
		{Name: "@", Type: "SOA", Data: "opaque", TTL: 600},
		{Name: "@", Type: "SRV", Data: "mail.example.com.", TTL: 600, Priority: u32(1), Weight: u32(1), Port: u32(1), Service: "_x", Protocol: "_tcp"},
		{Name: "@", Type: "CNAME", Data: "mail.example.com.", TTL: 600},
		{Name: "mail", Type: "MX", Data: "mail.example.com.", TTL: 600},
		{Name: "mail", Type: "A", Data: "999.1.1.1", TTL: 600},
		{Name: "mail", Type: "TXT", Data: "bad\nvalue", TTL: 600},
		{Name: "mail", Type: "CAA", Data: "example.com", TTL: 600, Flag: u32(256), Tag: "issue"},
	}
	for i, tc := range cases {
		if _, err := canonicalRecord(tc, false); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestCanonicalMutableSRV(t *testing.T) {
	t.Parallel()
	valid := apiRecord{Name: "_submission._tcp", Type: "SRV", Data: "mail.example.com.", TTL: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}
	if got, err := canonicalRecord(valid, false); err != nil || got != valid {
		t.Fatalf("valid SRV got=%#v err=%v", got, err)
	}
	unavailable := valid
	unavailable.Data = "."
	if _, err := canonicalRecord(unavailable, false); err != nil {
		t.Fatalf("unavailable-service target rejected: %v", err)
	}
	for name, mutate := range map[string]func(*apiRecord){
		"owner mismatch":   func(record *apiRecord) { record.Name = "_imap._tcp" },
		"owner wildcard":   func(record *apiRecord) { record.Name = "_submission._tcp.*.apps" },
		"service dot":      func(record *apiRecord) { record.Service = "_sub.mission" },
		"service hyphen":   func(record *apiRecord) { record.Service = "_-submission" },
		"protocol":         func(record *apiRecord) { record.Protocol = "_quic" },
		"missing priority": func(record *apiRecord) { record.Priority = nil },
		"missing weight":   func(record *apiRecord) { record.Weight = nil },
		"missing port":     func(record *apiRecord) { record.Port = nil },
		"target":           func(record *apiRecord) { record.Data = "mail.example.com" },
		"auxiliary":        func(record *apiRecord) { record.Tag = "issue" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if _, err := canonicalRecord(candidate, false); err == nil {
				t.Fatal("invalid SRV accepted")
			}
		})
	}
}

func TestSnapshotDeterminismAndIdentity(t *testing.T) {
	t.Parallel()
	a := apiRecord{RecordID: "id-a", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}
	b := apiRecord{RecordID: "id-b", Name: "_dmarc", Type: "TXT", Data: "v=DMARC1", TTL: 600}
	left, err := buildSnapshot("example.com", []apiRecord{a, b})
	if err != nil {
		t.Fatal(err)
	}
	right, err := buildSnapshot("example.com", []apiRecord{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshotsEqual(left, right) {
		t.Fatal("record order changed snapshot")
	}
	a.RecordID = "id-c"
	changed, err := buildSnapshot("example.com", []apiRecord{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Digest == left.Digest {
		t.Fatal("provider identity missing from snapshot")
	}
	if recordDigest(a) != recordDigest(left.Records[0]) && recordDigest(a) != recordDigest(left.Records[1]) {
		t.Fatal("logical digest unexpectedly included provider identity")
	}
}

func TestDuplicateProviderIDRejected(t *testing.T) {
	t.Parallel()
	r := apiRecord{RecordID: "same", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}
	if _, err := buildSnapshot("example.com", []apiRecord{r, r}); err == nil {
		t.Fatal("duplicate ID accepted")
	}
}

func TestCanonicalOrderAndWildcardName(t *testing.T) {
	t.Parallel()
	a := apiRecord{RecordID: "z-id", Name: "*.apps", Type: "A", Data: "192.0.2.1", TTL: 600}
	txt := apiRecord{RecordID: "a-id", Name: "@", Type: "TXT", Data: "value", TTL: 600}
	s, err := buildSnapshot("example.com", []apiRecord{txt, a})
	if err != nil {
		t.Fatal(err)
	}
	if s.Records[0].Type != "A" {
		t.Fatalf("sort was not type-first: %+v", s.Records)
	}
	bad := a
	bad.Name = "apps.*"
	if _, err := canonicalRecord(bad, true); err == nil {
		t.Fatal("non-leftmost wildcard accepted")
	}
}
