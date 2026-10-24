package godaddydns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
	extensioncore "github.com/gotthboard/gotth-extensions/pkg/extensions"
	extensionsv1 "github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testToken = "12345678901234567890123456789012"

func authContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	return metadata.NewIncomingContext(ctx, metadata.Pairs("x-gotth-service-token", testToken)), cancel
}

func baseService(client *providerClient, capabilities ...string) *Service {
	return &Service{config: Config{Schema: ConfigSchema, Environment: "ote", Zones: []string{"example.com"}, RecordTypes: []string{"A", "MX", "SRV", "TXT"}, TimeoutSeconds: 1}, binding: Binding{Capabilities: capabilities, ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", GrantSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SessionSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}, token: testToken, version: "dev", client: client}
}

func TestAuthorizeRequiresDeadlineTokenAndGrant(t *testing.T) {
	s := baseService(nil, "dns.records.observe")
	if err := s.authorize(context.Background(), "11111111-1111-1111-1111-111111111111", "dns.records.observe"); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing deadline: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.authorize(ctx, "11111111-1111-1111-1111-111111111111", "dns.records.observe"); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token: %v", err)
	}
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-gotth-service-token", testToken))
	if err := s.authorize(ctx, "11111111-1111-1111-1111-111111111111", "dns.records.delete"); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("missing capability: %v", err)
	}
	if err := s.authorize(ctx, "11111111-1111-1111-1111-111111111111", "dns.records.observe"); err != nil {
		t.Fatal(err)
	}
}

func TestCreateRecordEndToEndAgainstFakeProvider(t *testing.T) {
	var mu sync.Mutex
	records := []apiRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
		case http.MethodPost:
			if r.Header.Get("X-Request-Id") != "22222222-2222-2222-2222-222222222222" {
				t.Errorf("mutation operation ID not propagated: %q", r.Header.Get("X-Request-Id"))
			}
			var got apiRecord
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode create: %v", err)
			}
			got.RecordID = "created"
			records = append(records, got)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(got)
		default:
			t.Errorf("unexpected method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client := testClient(server)
	before := snap(t, records...)
	s := baseService(client, "dns.records.create")
	ctx, cancel := authContext()
	defer cancel()
	response, err := s.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, ExpectedLogicalCount: 0, Desired: &dnsv1.Record{Name: "_submission._tcp", Type: dnsv1.RecordType_RECORD_TYPE_SRV, Data: "mail.example.com.", Ttl: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || response.GetAffectedRecordId() != "created" || response.GetBeforeSnapshotSha256() != before.Digest || response.GetAfterSnapshotSha256() == before.Digest || response.GetResultingRecord().GetPort() != 587 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestServiceRejectsStaleAndNoopMutations(t *testing.T) {
	records := []apiRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writePage(t, w, records, 1, 1) }))
	defer server.Close()
	s := baseService(testClient(server), "dns.records.replace")
	ctx, cancel := authContext()
	defer cancel()
	before := snap(t, records...)
	request := &dnsv1.ReplaceRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, RecordId: "existing", ExpectedRecordSha256: recordDigest(records[0]), Desired: &dnsv1.Record{Name: "@", Type: dnsv1.RecordType_RECORD_TYPE_A, Data: "192.0.2.1", Ttl: 600}}
	if _, err := s.ReplaceRecord(ctx, request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("no-op replacement: %v", err)
	}
	request.ExpectedSnapshotSha256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	request.Desired.Data = "192.0.2.2"
	if _, err := s.ReplaceRecord(ctx, request); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("stale replacement: %v", err)
	}
}

func TestServiceRejectsConfiguredOutSRVBeforeHTTP(t *testing.T) {
	s := baseService(nil, "dns.records.create")
	s.config.RecordTypes = []string{"A"}
	ctx, cancel := authContext()
	defer cancel()
	_, err := s.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Desired: &dnsv1.Record{Name: "_submission._tcp", Type: dnsv1.RecordType_RECORD_TYPE_SRV, Data: "mail.example.com.", Ttl: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("configured-out SRV reached provider: %v", err)
	}
}

func TestManifestIsBounded(t *testing.T) {
	m, err := Manifest("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != ExtensionID || len(m.Interfaces) != 1 || len(m.Secrets) != 1 || len(m.Capabilities) != len(capabilities) {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if _, err := Manifest("1.0.0-alpha.2"); err == nil {
		t.Fatal("invalid version accepted")
	}
}

func TestFoundationNegotiation(t *testing.T) {
	manifest, err := Manifest("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	digest, err := extensioncore.ManifestDigest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	grant := extensioncore.Grant{Schema: extensioncore.GrantSchema, InstanceID: "11111111-1111-1111-1111-111111111111", ExtensionID: ExtensionID, ManifestDigest: digest, Capabilities: []string{"dns.records.observe"}, Interfaces: []extensioncore.InterfaceGrant{{Name: Interface, Major: 1, Minor: 0}}, Secrets: []string{SecretSlot}}
	host := extensioncore.HostProfile{Protocols: []extensioncore.VersionRange{{Name: extensioncore.ControlName, Major: 1, MinMinor: 0, MaxMinor: 0}}, Interfaces: []extensioncore.VersionRange{{Name: Interface, Major: 1, MinMinor: 0, MaxMinor: 0}}}
	session, err := extensioncore.Negotiate(manifest, grant, host)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.Capabilities) != 1 || session.Capabilities[0] != "dns.records.observe" || session.Fingerprint == "" {
		t.Fatalf("negotiation: %+v", session)
	}
}

func TestControlHandshakeHealthAndNewService(t *testing.T) {
	runtime := RuntimeConfig{Config: Config{Schema: ConfigSchema, Environment: "ote", Zones: []string{"example.com"}, RecordTypes: []string{"A"}, TimeoutSeconds: 1}, Binding: Binding{ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", GrantSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SessionSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Capabilities: []string{"dns.records.observe"}}, ServiceToken: testToken, PAT: []byte("0123456789abcdef0123456789abcdef"), Version: "dev"}
	s, err := NewService(runtime)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := authContext()
	defer cancel()
	handshake, err := s.Handshake(ctx, &extensionsv1.HandshakeRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", Challenge: make([]byte, 32), ExpectedExtensionId: ExtensionID, ExpectedManifestSha256: runtime.Binding.ManifestSHA256, ExpectedGrantSha256: runtime.Binding.GrantSHA256, ExpectedSessionSha256: runtime.Binding.SessionSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if len(handshake.GetCapabilities()) != 1 || handshake.GetCapabilities()[0] != "dns.records.observe" {
		t.Fatalf("grant not reflected: %+v", handshake)
	}
	health, err := s.Health(ctx, &extensionsv1.HealthRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", SessionSha256: runtime.Binding.SessionSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if health.GetState() != extensionsv1.HealthState_HEALTH_STATE_READY {
		t.Fatalf("health: %+v", health)
	}
	if _, err := s.Health(ctx, &extensionsv1.HealthRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", SessionSha256: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("session mismatch accepted: %v", err)
	}
}

func TestObserveReplaceDeleteEndToEnd(t *testing.T) {
	var mu sync.Mutex
	records := []apiRecord{{RecordID: "target", Name: "_submission._tcp", Type: "SRV", Data: "mail.example.com.", TTL: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}, {RecordID: "keep", Name: "txt", Type: "TXT", Data: "keep", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
		case http.MethodPut:
			var got apiRecord
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode replacement: %v", err)
			}
			got.RecordID = "target"
			records[0] = got
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(got)
		case http.MethodDelete:
			records = records[1:]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	s := baseService(testClient(server), "dns.records.delete", "dns.records.observe", "dns.records.replace")
	ctx, cancel := authContext()
	defer cancel()
	observed, err := s.ObserveZone(ctx, &dnsv1.ObserveZoneRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", Zone: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(observed.GetSnapshot().GetRecords()) != 2 {
		t.Fatalf("observation: %+v", observed)
	}
	before := snap(t, records...)
	previous := records[0]
	replaced, err := s.ReplaceRecord(ctx, &dnsv1.ReplaceRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, RecordId: "target", ExpectedRecordSha256: recordDigest(previous), Desired: &dnsv1.Record{Name: "_submission._tcp", Type: dnsv1.RecordType_RECORD_TYPE_SRV, Data: "mail.example.com.", Ttl: 600, Priority: u32(10), Weight: u32(20), Port: u32(465), Service: "_submission", Protocol: "_tcp"}})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED {
		t.Fatalf("replacement: %+v", replaced)
	}
	mu.Lock()
	afterReplaceRecords := append([]apiRecord(nil), records...)
	mu.Unlock()
	afterReplace := snap(t, afterReplaceRecords...)
	current, _ := findRecord(afterReplace, "target")
	deleted, err := s.DeleteRecord(ctx, &dnsv1.DeleteRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "33333333-3333-3333-3333-333333333333", Zone: "example.com", ExpectedSnapshotSha256: afterReplace.Digest, RecordId: "target", ExpectedRecordSha256: recordDigest(current)})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || deleted.GetPreviousRecord().GetPort() != 465 || deleted.GetPreviousRecord().GetService() != "_submission" {
		t.Fatalf("deletion: %+v", deleted)
	}
}

func TestConcurrentMutationsAreSerialized(t *testing.T) {
	var mu sync.Mutex
	posts := 0
	records := []apiRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
			return
		}
		if r.Method == http.MethodPost {
			posts++
			var got apiRecord
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode create: %v", err)
			}
			got.RecordID = "created"
			records = append(records, got)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(got)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer server.Close()
	s := baseService(testClient(server), "dns.records.create")
	before := snap(t, records...)
	type result struct {
		response *dnsv1.MutationResponse
		err      error
	}
	results := make(chan result, 2)
	for i, data := range []string{"192.0.2.2", "192.0.2.3"} {
		go func(i int, data string) {
			ctx, cancel := authContext()
			defer cancel()
			op := "22222222-2222-2222-2222-222222222222"
			if i == 1 {
				op = "33333333-3333-3333-3333-333333333333"
			}
			response, err := s.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: op, Zone: "example.com", ExpectedSnapshotSha256: before.Digest, ExpectedLogicalCount: 0, Desired: &dnsv1.Record{Name: "mail", Type: dnsv1.RecordType_RECORD_TYPE_A, Data: data, Ttl: 600}})
			results <- result{response, err}
		}(i, data)
	}
	applied, stale := 0, 0
	for i := 0; i < 2; i++ {
		got := <-results
		if got.err == nil && got.response.GetOutcome() == dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED {
			applied++
		} else if status.Code(got.err) == codes.FailedPrecondition {
			stale++
		} else {
			t.Fatalf("unexpected concurrent result: response=%+v err=%v", got.response, got.err)
		}
	}
	mu.Lock()
	gotPosts := posts
	mu.Unlock()
	if applied != 1 || stale != 1 || gotPosts != 1 {
		t.Fatalf("serialization failed: applied=%d stale=%d posts=%d", applied, stale, gotPosts)
	}
}

func TestLostMutationResponseReconcilesApplied(t *testing.T) {
	var mu sync.Mutex
	records := []apiRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
			return
		}
		if r.Method == http.MethodPost {
			var got apiRecord
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode create: %v", err)
			}
			got.RecordID = "created-after-loss"
			records = append(records, got)
			panic(http.ErrAbortHandler)
		}
	}))
	defer server.Close()
	s := baseService(testClient(server), "dns.records.create")
	before := snap(t, records...)
	ctx, cancel := authContext()
	defer cancel()
	response, err := s.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, ExpectedLogicalCount: 0, Desired: &dnsv1.Record{Name: "_submission._tcp", Type: dnsv1.RecordType_RECORD_TYPE_SRV, Data: "mail.example.com.", Ttl: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || response.GetReason() != dnsv1.MutationReason_MUTATION_REASON_RECONCILED || response.GetAffectedRecordId() != "created-after-loss" {
		t.Fatalf("lost response was not reconciled: %+v", response)
	}
}

func TestMutationDeadlineExhaustionRetainsReconciliationBudget(t *testing.T) {
	var mu sync.Mutex
	var mutationDeadlineExpired atomic.Bool
	var gets atomic.Int32
	records := []apiRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			if gets.Add(1) == 4 {
				time.Sleep(350 * time.Millisecond)
			}
			writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
			return
		}
		var got apiRecord
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode create: %v", err)
			return
		}
		got.RecordID = "created-after-deadline"
		records = append(records, got)
		<-r.Context().Done()
		mutationDeadlineExpired.Store(true)
	}))
	defer server.Close()
	client := testClient(server)
	client.timeout = 400 * time.Millisecond
	client.http.Timeout = time.Second
	s := baseService(client, "dns.records.create")
	before := snap(t, records...)
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-gotth-service-token", testToken))
	response, err := s.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, ExpectedLogicalCount: 0, Desired: &dnsv1.Record{Name: "mail", Type: dnsv1.RecordType_RECORD_TYPE_A, Data: "192.0.2.2", Ttl: 600}})
	if err != nil {
		t.Fatal(err)
	}
	if !mutationDeadlineExpired.Load() || gets.Load() != 4 || response.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || response.GetReason() != dnsv1.MutationReason_MUTATION_REASON_RECONCILED || response.GetAffectedRecordId() != "created-after-deadline" {
		t.Fatalf("deadline-exhausted mutation was not reconciled: expired=%v gets=%d response=%+v", mutationDeadlineExpired.Load(), gets.Load(), response)
	}
}

func TestMutableRecordFixturesAgainstFakeProvider(t *testing.T) {
	fixtures := []struct {
		name    string
		desired *dnsv1.Record
	}{
		{"A", &dnsv1.Record{Name: "mail", Type: dnsv1.RecordType_RECORD_TYPE_A, Data: "192.0.2.10", Ttl: 600}},
		{"AAAA", &dnsv1.Record{Name: "mail", Type: dnsv1.RecordType_RECORD_TYPE_AAAA, Data: "2001:db8::10", Ttl: 600}},
		{"CAA", &dnsv1.Record{Name: "@", Type: dnsv1.RecordType_RECORD_TYPE_CAA, Data: "letsencrypt.org", Ttl: 600, Flag: u32(0), Tag: "issue"}},
		{"CNAME", &dnsv1.Record{Name: "autoconfig", Type: dnsv1.RecordType_RECORD_TYPE_CNAME, Data: "mail.example.com.", Ttl: 600}},
		{"MX", &dnsv1.Record{Name: "@", Type: dnsv1.RecordType_RECORD_TYPE_MX, Data: "mail.example.com.", Ttl: 600, Priority: u32(10)}},
		{"TXT", &dnsv1.Record{Name: "_dmarc", Type: dnsv1.RecordType_RECORD_TYPE_TXT, Data: "v=DMARC1; p=reject", Ttl: 600}},
		{"SRV", &dnsv1.Record{Name: "_submission._tcp", Type: dnsv1.RecordType_RECORD_TYPE_SRV, Data: "mail.example.com.", Ttl: 600, Priority: u32(10), Weight: u32(20), Port: u32(587), Service: "_submission", Protocol: "_tcp"}},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var mu sync.Mutex
			var posted apiRecord
			posts := 0
			records := []apiRecord{{RecordID: "keep", Name: "@", Type: "NS", Data: "ns1.example.com.", TTL: 3600}}
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch r.Method {
				case http.MethodGet:
					writePage(t, w, append([]apiRecord(nil), records...), len(records), 1)
				case http.MethodPost:
					posts++
					if r.URL.Path != "/v3/domains/zones/example.com/dns-records" || r.Header.Get("X-Request-Id") != "22222222-2222-2222-2222-222222222222" || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("unexpected mutation request: %s headers=%v", r.URL.Path, r.Header)
					}
					if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
						t.Errorf("decode %s fixture: %v", fixture.name, err)
					}
					posted.RecordID = "created-" + fixture.name
					records = append(records, posted)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(posted)
				default:
					t.Errorf("unexpected method %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()
			s := baseService(testClient(server), "dns.records.create")
			s.config.RecordTypes = []string{"A", "AAAA", "CAA", "CNAME", "MX", "SRV", "TXT"}
			before := snap(t, records...)
			ctx, cancel := authContext()
			defer cancel()
			request := &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-1111-1111-111111111111", OperationId: "22222222-2222-2222-2222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: before.Digest, ExpectedLogicalCount: 0, Desired: fixture.desired}
			response, err := s.CreateRecord(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			want, err := recordFromProto(fixture.desired, false)
			if err != nil {
				t.Fatal(err)
			}
			got, err := recordFromProto(response.GetResultingRecord(), true)
			if err != nil {
				t.Fatal(err)
			}
			if response.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || response.GetReason() != dnsv1.MutationReason_MUTATION_REASON_RECONCILED || response.GetAffectedRecordId() != "created-"+fixture.name || recordDigest(got) != recordDigest(want) || posts != 1 {
				t.Fatalf("%s fixture result=%+v posted=%+v posts=%d", fixture.name, response, posted, posts)
			}
			if _, err := s.CreateRecord(ctx, request); status.Code(err) != codes.FailedPrecondition {
				t.Fatalf("duplicate %s operation was not rejected as stale: %v", fixture.name, err)
			}
			if posts != 1 {
				t.Fatalf("duplicate %s operation caused another provider mutation", fixture.name)
			}
		})
	}
}

func TestRollbackMaterialSemantics(t *testing.T) {
	keep := apiRecord{RecordID: "keep", Name: "txt", Type: "TXT", Data: "keep", TTL: 600}
	previous := apiRecord{RecordID: "target", Name: "mail", Type: "A", Data: "192.0.2.1", TTL: 600}
	desired := apiRecord{Name: "mail", Type: "A", Data: "192.0.2.2", TTL: 600}
	resulting := desired
	resulting.RecordID = previous.RecordID
	before := snap(t, keep, previous)

	created := desired
	created.RecordID = "created"
	create := classifyCreate(before, snap(t, keep, previous, created), desired, providerAccepted, nil)
	if create.GetResultingRecord().GetRecordId() != "created" || create.GetPreviousRecord() != nil {
		t.Fatalf("create inverse must delete the observed provider ID: %+v", create)
	}
	replace := classifyReplace(before, snap(t, keep, resulting), previous, desired, providerAccepted, nil)
	if replace.GetPreviousRecord().GetRecordId() != "target" || replace.GetResultingRecord().GetRecordId() != "target" {
		t.Fatalf("replace inverse must preserve exact prior state and provider ID: %+v", replace)
	}
	deleted := classifyDelete(before, snap(t, keep), previous, providerAccepted, nil)
	if deleted.GetPreviousRecord().GetRecordId() != "target" || deleted.GetResultingRecord() != nil {
		t.Fatalf("delete inverse must return prior logical state without claiming ID preservation: %+v", deleted)
	}
}
