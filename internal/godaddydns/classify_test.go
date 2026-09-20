package godaddydns

import (
	"errors"
	"testing"

	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
)

func snap(t *testing.T, records ...apiRecord) snapshot {
	t.Helper()
	s, err := buildSnapshot("example.com", records)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateClassification(t *testing.T) {
	t.Parallel()
	old := apiRecord{RecordID: "old", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}
	desired := apiRecord{Name: "mail", Type: "A", Data: "192.0.2.2", TTL: 600}
	added := desired
	added.RecordID = "new"
	before := snap(t, old)
	after := snap(t, old, added)
	r := classifyCreate(before, after, desired, providerUnavailable, nil)
	if r.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || r.AffectedRecordId != "new" {
		t.Fatalf("unexpected applied result: %+v", r)
	}
	r = classifyCreate(before, before, desired, providerRejected, nil)
	if r.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_NOT_APPLIED || r.Reason != dnsv1.MutationReason_MUTATION_REASON_PROVIDER_REJECTED {
		t.Fatalf("unexpected rejected result: %+v", r)
	}
	unrelated := snap(t, old, apiRecord{RecordID: "other", Name: "x", Type: "A", Data: "192.0.2.3", TTL: 600})
	r = classifyCreate(before, unrelated, desired, providerAccepted, nil)
	if r.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN {
		t.Fatalf("concurrent change accepted: %+v", r)
	}
	r = classifyCreate(before, snapshot{}, desired, providerAccepted, errors.New("observe"))
	if r.Reason != dnsv1.MutationReason_MUTATION_REASON_OBSERVATION_FAILED {
		t.Fatalf("observation failure hidden: %+v", r)
	}
}

func TestReplaceAndDeleteClassification(t *testing.T) {
	t.Parallel()
	old := apiRecord{RecordID: "id", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}
	keep := apiRecord{RecordID: "keep", Name: "txt", Type: "TXT", Data: "hello", TTL: 600}
	desired := apiRecord{Name: "@", Type: "A", Data: "192.0.2.2", TTL: 600}
	changed := desired
	changed.RecordID = "id"
	before := snap(t, old, keep)
	replaced := snap(t, changed, keep)
	deleted := snap(t, keep)
	r := classifyReplace(before, replaced, old, desired, providerAccepted, nil)
	if r.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || r.ResultingRecord.GetData() != desired.Data {
		t.Fatalf("replacement not reconciled: %+v", r)
	}
	d := classifyDelete(before, deleted, old, providerUnavailable, nil)
	if d.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || d.PreviousRecord.GetRecordId() != "id" {
		t.Fatalf("delete not reconciled: %+v", d)
	}
	concurrent := snap(t, changed)
	r = classifyReplace(before, concurrent, old, desired, providerAccepted, nil)
	if r.Outcome != dnsv1.MutationOutcome_MUTATION_OUTCOME_UNKNOWN {
		t.Fatal("unrelated deletion not detected")
	}
}
