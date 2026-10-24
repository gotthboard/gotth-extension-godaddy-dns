package godaddydns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(server *httptest.Server) *providerClient {
	client := server.Client()
	client.Timeout = 100 * time.Millisecond
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &providerClient{base: server.URL + "/v3/domains", pat: []byte("secret-pat-value"), http: client, timeout: 100 * time.Millisecond}
}

func writePage(t *testing.T, w http.ResponseWriter, records []apiRecord, total, pages int) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pageResponse{Items: records, TotalItems: &total, TotalPages: &pages, Links: []pageLink{{Rel: "self", Href: "https://api.godaddy.invalid/page"}}})
}

func TestObserveTwoStableWalksAndHeaders(t *testing.T) {
	var calls atomic.Int32
	records := []apiRecord{{RecordID: "id-a", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}, {RecordID: "id-ns", Name: "@", Type: "NS", Data: "ns1.example.com.", TTL: 3600}}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v3/domains/zones/example.com/dns-records" || r.URL.Query().Get("page") != "1" || r.URL.Query().Get("pageSize") != "100" || r.URL.Query().Get("totalRequired") != "true" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if r.Header.Get("Authorization") != "Bearer secret-pat-value" || r.Header.Get("X-Request-Id") == "" {
			t.Error("required headers missing")
		}
		writePage(t, w, records, len(records), 1)
	}))
	defer server.Close()
	snap, err := testClient(server).observe(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || len(snap.Records) != 2 || snap.Digest == "" {
		t.Fatalf("unexpected observation: calls=%d records=%d", calls.Load(), len(snap.Records))
	}
}

func TestObserveRejectsUnstableDuplicateAndUnknownJSON(t *testing.T) {
	t.Run("unstable", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := calls.Add(1)
			data := "192.0.2.1"
			if n%2 == 0 {
				data = "192.0.2.2"
			}
			writePage(t, w, []apiRecord{{RecordID: "id", Name: "@", Type: "A", Data: data, TTL: 600}}, 1, 1)
		}))
		defer server.Close()
		if _, err := testClient(server).observe(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111"); err == nil {
			t.Fatal("unstable snapshot accepted")
		}
	})
	for name, body := range map[string]string{
		"duplicate": `{"items":[],"items":[],"totalItems":0,"totalPages":1,"links":[]}`,
		"unknown":   `{"items":[],"totalItems":0,"totalPages":1,"links":[],"surprise":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			if _, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111"); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		})
	}
}

func TestWalkPagination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var records []apiRecord
		if r.URL.Query().Get("page") == "1" {
			for i := 0; i < 100; i++ {
				records = append(records, apiRecord{RecordID: fmt.Sprintf("id-%03d", i), Name: fmt.Sprintf("host%03d", i), Type: "A", Data: fmt.Sprintf("192.0.2.%d", i+1), TTL: 600})
			}
		} else {
			records = []apiRecord{{RecordID: "id-100", Name: "host100", Type: "A", Data: "192.0.2.101", TTL: 600}}
		}
		writePage(t, w, records, 101, 2)
	}))
	defer server.Close()
	snap, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Records) != 101 {
		t.Fatalf("got %d records", len(snap.Records))
	}
}

func TestWalkAcceptsDocumentedEmptyPagination(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"items\":[],\"links\":[]}"))
	}))
	defer server.Close()
	snapshot, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Records) != 0 || snapshot.Digest == "" {
		t.Fatalf("unexpected empty snapshot: %+v", snapshot)
	}
}

func TestWalkRejectsNullEmptyPaginationArrays(t *testing.T) {
	for _, body := range []string{"{\"items\":null,\"links\":[]}", "{\"items\":[],\"links\":null}"} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		_, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
		server.Close()
		if err == nil {
			t.Fatalf("null pagination array accepted: %s", body)
		}
	}
}

func TestWalkRejectsNullTotalsAndNonEmptyEmptyLinks(t *testing.T) {
	bodies := []string{
		"{\"items\":[],\"totalItems\":null,\"totalPages\":null,\"links\":[]}",
		"{\"items\":[],\"totalItems\":1,\"links\":[]}",
		"{\"items\":[],\"links\":[{\"rel\":\"self\",\"href\":\"https://example.invalid/\"}]}",
	}
	for _, body := range bodies {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		_, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
		server.Close()
		if err == nil {
			t.Fatalf("invalid empty pagination accepted: %s", body)
		}
	}
}

func TestRejectedObservationsCloseBodiesAndRecover(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(strings.Repeat("x", 8192)))
			return
		}
		writePage(t, w, []apiRecord{{RecordID: "id", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}, 1, 1)
	}))
	defer server.Close()
	client := testClient(server)
	transport := client.http.Transport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = 2
	client.http.Transport = transport
	for i := 0; i < 2; i++ {
		if _, err := client.walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111"); err == nil {
			t.Fatal("rejected observation accepted")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.walk(ctx, "example.com", "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("client did not recover after rejected responses: %v", err)
	}
}

func TestMutationRequestAndStatusClassification(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		status       int
		want         providerResult
	}{
		{"create", http.MethodPost, http.StatusCreated, providerAccepted}, {"replace", http.MethodPut, http.StatusOK, providerAccepted}, {"delete", http.MethodDelete, http.StatusNoContent, providerAccepted},
		{"bad-request", http.MethodPost, 400, providerRejected}, {"unauthorized", http.MethodPost, 401, providerRejected}, {"forbidden", http.MethodPost, 403, providerRejected}, {"missing", http.MethodPost, 404, providerRejected}, {"conflict", http.MethodPost, 409, providerRejected}, {"unprocessable", http.MethodPost, 422, providerRejected},
		{"rate", http.MethodPost, 429, providerUnavailable}, {"server", http.MethodPost, 500, providerUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || !strings.HasPrefix(r.URL.Path, "/v3/domains/zones/example.com/dns-records") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if tc.status == http.StatusCreated || tc.status == http.StatusOK {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(tc.status)
				if tc.status == http.StatusCreated || tc.status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(apiRecord{RecordID: "new-id", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600})
				}
			}))
			defer server.Close()
			record := apiRecord{Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}
			id := ""
			var body *apiRecord
			if tc.method != http.MethodDelete {
				body = &record
			}
			if tc.method != http.MethodPost {
				id = "old-id"
			}
			if got := testClient(server).mutate(context.Background(), tc.method, "example.com", id, "11111111-1111-1111-1111-111111111111", body); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestProductionTransportIsFixedAndBounded(t *testing.T) {
	client := newProviderClient(Config{Environment: "production", TimeoutSeconds: 1}, []byte("0123456789abcdef"))
	defer client.close()
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected transport")
	}
	if client.base != "https://api.godaddy.com/v3/domains" || transport.Proxy != nil || !transport.DisableCompression || transport.MaxConnsPerHost != 2 || transport.MaxResponseHeaderBytes != 64<<10 || transport.TLSClientConfig.MinVersion == 0 {
		t.Fatalf("unbounded production client: %+v", transport)
	}
	ote := newProviderClient(Config{Environment: "ote", TimeoutSeconds: 1}, []byte("0123456789abcdef"))
	defer ote.close()
	if ote.base != "https://api.ote-godaddy.com/v3/domains" {
		t.Fatalf("OTE base: %s", ote.base)
	}
}

func TestMutationHonorsCancellationAndMediaType(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		defer server.Close()
		defer close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if got := testClient(server).mutate(ctx, http.MethodPost, "example.com", "", "11111111-1111-1111-1111-111111111111", &apiRecord{Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}); got != providerUnavailable {
			t.Fatalf("canceled result: %v", got)
		}
	})
	t.Run("media-type", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"recordId":"id"}`))
		}))
		defer server.Close()
		if got := testClient(server).mutate(context.Background(), http.MethodPost, "example.com", "", "11111111-1111-1111-1111-111111111111", &apiRecord{Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}); got != providerUnavailable {
			t.Fatalf("wrong media type accepted: %v", got)
		}
	})
}

func TestClientDoesNotFollowRedirect(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	c := testClient(redirect)
	if got := c.mutate(context.Background(), http.MethodPost, "example.com", "", "11111111-1111-1111-1111-111111111111", &apiRecord{Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}); got != providerUnavailable {
		t.Fatalf("redirect result %v", got)
	}
	if targetCalls.Load() != 0 {
		t.Fatal("redirect followed")
	}
}

func TestWalkRejectsInconsistentPaginationAndProviderText(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"page-count", func(w http.ResponseWriter, r *http.Request) {
			writePage(t, w, []apiRecord{{RecordID: "id", Name: "mail", Type: "A", Data: "192.0.2.1", TTL: 600}}, 101, 1)
		}},
		{"too-many-records", func(w http.ResponseWriter, r *http.Request) { writePage(t, w, nil, 1001, 10) }},
		{"missing-links", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			total, pages := 0, 1
			_ = json.NewEncoder(w).Encode(pageResponse{Items: nil, TotalItems: &total, TotalPages: &pages})
		}},
		{"remote-error-text", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("secret hostile provider body"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(tc.handler)
			defer server.Close()
			_, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111")
			if err == nil {
				t.Fatal("invalid provider pagination accepted")
			}
			if strings.Contains(err.Error(), "secret hostile provider body") {
				t.Fatalf("remote provider text escaped: %v", err)
			}
		})
	}
}

func TestWalkRejectsChangingTotalsBetweenPages(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		records := make([]apiRecord, 100)
		for i := range records {
			records[i] = apiRecord{RecordID: fmt.Sprintf("id-%03d", i), Name: fmt.Sprintf("host%03d", i), Type: "A", Data: fmt.Sprintf("192.0.2.%d", i+1), TTL: 600}
		}
		if r.URL.Query().Get("page") == "1" {
			writePage(t, w, records, 101, 2)
			return
		}
		writePage(t, w, nil, 100, 1)
	}))
	defer server.Close()
	if _, err := testClient(server).walk(context.Background(), "example.com", "11111111-1111-1111-1111-111111111111"); err == nil {
		t.Fatal("pagination totals changed without rejection")
	}
}

func TestMutationClassifiesTLSFailureWithoutProviderText(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("remote diagnostic must not escape"))
	}))
	defer server.Close()
	client := &providerClient{base: server.URL + "/v3/domains", pat: []byte("secret-pat-value"), http: &http.Client{Timeout: time.Second}, timeout: time.Second}
	got := client.mutate(context.Background(), http.MethodPost, "example.com", "", "11111111-1111-1111-1111-111111111111", &apiRecord{Name: "mail", Type: "A", Data: "192.0.2.1", TTL: 600})
	if got != providerUnavailable {
		t.Fatalf("TLS failure classified as %v", got)
	}
}
