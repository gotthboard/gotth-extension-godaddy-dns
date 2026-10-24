package process_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns"
	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
	extensioncore "github.com/gotthboard/gotth-extensions/pkg/extensions"
	extensionsv1 "github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthenticatedUnixProcess(t *testing.T) {
	binary := os.Getenv("GOTTH_PROVIDER_BINARY")
	if binary == "" {
		t.Skip("GOTTH_PROVIDER_BINARY is required for process integration")
	}
	root := t.TempDir()
	secretDir := filepath.Join(root, "secrets")
	runtimeDir := filepath.Join(root, "run")
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(secretDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := godaddydns.ManifestDigest("dev")
	if err != nil {
		t.Fatal(err)
	}
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	binding, _ := json.Marshal(godaddydns.Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: manifest, GrantSHA256: digest, SessionSHA256: digest, Interfaces: []extensioncore.InterfaceGrant{{Name: godaddydns.Interface, Major: 1, Minor: 0}}, Capabilities: []string{"dns.records.observe"}})
	config := []byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["A","SRV"],"timeout_seconds":1}`)
	paths := map[string][]byte{"config": config, "binding": binding, "token": []byte("12345678901234567890123456789012"), filepath.Join("secrets", godaddydns.SecretSlot): []byte("0123456789abcdef0123456789abcdef")}
	for name, body := range paths {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socket := filepath.Join(runtimeDir, "provider.sock")
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "GOTTH_EXTENSION_CONFIG_FILE="+filepath.Join(root, "config"), "GOTTH_EXTENSION_BINDING_FILE="+filepath.Join(root, "binding"), "GOTTH_EXTENSION_SECRET_DIR="+secretDir, "GOTTH_EXTENSION_SERVICE_TOKEN_FILE="+filepath.Join(root, "token"), "GOTTH_EXTENSION_SOCKET="+socket)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(syscall.SIGTERM); _ = cmd.Wait() }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Lstat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider socket did not appear")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if info, err := os.Lstat(socket); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket permissions: %v %v", info, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.NewClient("passthrough:///provider", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := extensionsv1.NewExtensionControlClient(conn)
	request := &extensionsv1.HealthRequest{CorrelationId: "22222222-2222-4222-8222-222222222222", SessionSha256: digest}
	if _, err := client.Health(ctx, request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated call: %v", err)
	}
	authed := metadata.AppendToOutgoingContext(ctx, "x-gotth-service-token", "12345678901234567890123456789012")
	duplicated := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-gotth-service-token", "12345678901234567890123456789012", "x-gotth-service-token", "12345678901234567890123456789012"))
	if _, err := client.Health(duplicated, request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("duplicate authentication metadata accepted: %v", err)
	}
	health, err := client.Health(authed, request)
	if err != nil {
		t.Fatal(err)
	}
	if health.GetState() != extensionsv1.HealthState_HEALTH_STATE_READY {
		t.Fatalf("health: %+v", health)
	}
	challenge := bytes.Repeat([]byte{0x5a}, 32)
	handshake, err := client.Handshake(authed, &extensionsv1.HandshakeRequest{CorrelationId: "33333333-3333-4333-8333-333333333333", Challenge: challenge, ExpectedExtensionId: godaddydns.ExtensionID, ExpectedManifestSha256: manifest, ExpectedGrantSha256: digest, ExpectedSessionSha256: digest})
	if err != nil || !bytes.Equal(handshake.GetChallenge(), challenge) || len(handshake.GetInterfaces()) != 1 || handshake.GetInterfaces()[0].GetName() != godaddydns.Interface || handshake.GetInterfaces()[0].GetMajor() != 1 || handshake.GetInterfaces()[0].GetMinor() != 0 {
		t.Fatalf("handshake=%+v err=%v", handshake, err)
	}
}

type providerRecord struct {
	RecordID string `json:"recordId,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Data     string `json:"data"`
	TTL      uint32 `json:"ttl"`
}

func TestFakeProviderDNSOverCompiledProcess(t *testing.T) {
	sourceRoot := os.Getenv("GOTTH_PROVIDER_SOURCE_ROOT")
	if sourceRoot == "" {
		t.Skip("GOTTH_PROVIDER_SOURCE_ROOT is required for compiled fake-provider integration")
	}
	var mu sync.Mutex
	records := []providerRecord{{RecordID: "existing", Name: "@", Type: "A", Data: "192.0.2.1", TTL: 600}}
	gets, posts := 0, 0
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/v3/domains/zones/example.com/dns-records" || r.Header.Get("Authorization") != "Bearer 0123456789abcdef0123456789abcdef" {
			t.Errorf("unexpected provider request: %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			gets++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(struct {
				Items      []providerRecord    `json:"items"`
				TotalItems int                 `json:"totalItems"`
				TotalPages int                 `json:"totalPages"`
				Links      []map[string]string `json:"links"`
			}{Items: append([]providerRecord(nil), records...), TotalItems: len(records), TotalPages: 1, Links: []map[string]string{{"rel": "self", "href": r.URL.String()}}})
		case http.MethodPost:
			posts++
			var record providerRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Errorf("decode provider create: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			record.RecordID = "created"
			records = append(records, record)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(record)
		default:
			t.Errorf("unexpected provider method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer provider.Close()

	buildRoot := t.TempDir()
	binary := filepath.Join(buildRoot, "gotth-extension-godaddy-dns")
	build := exec.Command("go", "build", "-trimpath", "-tags=processintegration", "-o", binary, "./cmd/gotth-extension-godaddy-dns")
	build.Dir = sourceRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process binary: %v: %s", err, output)
	}
	caFile := filepath.Join(buildRoot, "provider-ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	if err := os.WriteFile(caFile, certificate, 0o600); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(buildRoot, "runtime")
	secretDir := filepath.Join(root, "secrets")
	runtimeDir := filepath.Join(root, "run")
	for _, dir := range []string{root, secretDir, runtimeDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := godaddydns.ManifestDigest("dev")
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	binding, err := json.Marshal(godaddydns.Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: manifest, GrantSHA256: digest, SessionSHA256: digest, Interfaces: []extensioncore.InterfaceGrant{{Name: godaddydns.Interface, Major: 1, Minor: 0}}, Capabilities: []string{"dns.records.create", "dns.records.observe"}})
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"config":  []byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["A","TXT"],"timeout_seconds":1}`),
		"binding": binding,
		"token":   []byte("12345678901234567890123456789012"),
		filepath.Join("secrets", godaddydns.SecretSlot): []byte("0123456789abcdef0123456789abcdef"),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	socket := filepath.Join(runtimeDir, "provider.sock")
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "GOTTH_PROCESS_TEST_PROVIDER_BASE="+provider.URL+"/v3/domains", "SSL_CERT_FILE="+caFile, "GOTTH_EXTENSION_CONFIG_FILE="+filepath.Join(root, "config"), "GOTTH_EXTENSION_BINDING_FILE="+filepath.Join(root, "binding"), "GOTTH_EXTENSION_SECRET_DIR="+secretDir, "GOTTH_EXTENSION_SERVICE_TOKEN_FILE="+filepath.Join(root, "token"), "GOTTH_EXTENSION_SOCKET="+socket)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(syscall.SIGTERM); _ = cmd.Wait() }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Lstat(socket); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider socket did not appear")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.NewClient("passthrough:///provider", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-gotth-service-token", "12345678901234567890123456789012")
	dns := dnsv1.NewDNSRecordsClient(conn)
	observed, err := dns.ObserveZone(ctx, &dnsv1.ObserveZoneRequest{CorrelationId: "11111111-1111-4111-8111-111111111111", Zone: "example.com"})
	if err != nil || len(observed.GetSnapshot().GetRecords()) != 1 || observed.GetSnapshot().GetRecords()[0].GetRecordId() != "existing" {
		t.Fatalf("observe=%+v err=%v", observed, err)
	}
	created, err := dns.CreateRecord(ctx, &dnsv1.CreateRecordRequest{CorrelationId: "11111111-1111-4111-8111-111111111111", OperationId: "22222222-2222-4222-8222-222222222222", Zone: "example.com", ExpectedSnapshotSha256: observed.GetSnapshot().GetSha256(), ExpectedLogicalCount: 0, Desired: &dnsv1.Record{Name: "_process", Type: dnsv1.RecordType_RECORD_TYPE_TXT, Data: "compiled process", Ttl: 600}})
	if err != nil || created.GetOutcome() != dnsv1.MutationOutcome_MUTATION_OUTCOME_APPLIED || created.GetAffectedRecordId() != "created" {
		t.Fatalf("create=%+v err=%v", created, err)
	}
	mu.Lock()
	gotGets, gotPosts := gets, posts
	mu.Unlock()
	if gotGets != 6 || gotPosts != 1 {
		t.Fatalf("provider calls: gets=%d posts=%d", gotGets, gotPosts)
	}
}

func TestStartupFailureIsRedacted(t *testing.T) {
	binary := os.Getenv("GOTTH_PROVIDER_BINARY")
	if binary == "" {
		t.Skip("GOTTH_PROVIDER_BINARY is required for process integration")
	}
	marker := "must-not-escape-runtime-secret"
	cmd := exec.Command(binary)
	cmd.Env = []string{
		"GOTTH_EXTENSION_CONFIG_FILE=/missing/" + marker,
		"GOTTH_EXTENSION_BINDING_FILE=/missing/binding",
		"GOTTH_EXTENSION_SECRET_DIR=/missing/secrets",
		"GOTTH_EXTENSION_SERVICE_TOKEN_FILE=/missing/token",
		"GOTTH_EXTENSION_SOCKET=/missing/provider.sock",
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("invalid runtime unexpectedly started")
	}
	if strings.Contains(string(output), marker) || string(output) != "gotth-extension-godaddy-dns: startup failed\n" {
		t.Fatalf("startup failure was not fixed and redacted: %q", output)
	}
}
