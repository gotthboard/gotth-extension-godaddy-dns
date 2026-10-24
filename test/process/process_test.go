package process_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gotthboard/gotth-extension-godaddy-dns/internal/godaddydns"
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
	binding, _ := json.Marshal(godaddydns.Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: manifest, GrantSHA256: digest, SessionSHA256: digest, Capabilities: []string{"dns.records.observe"}})
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
	request := &extensionsv1.HealthRequest{CorrelationId: "22222222-2222-2222-2222-222222222222", SessionSha256: digest}
	if _, err := client.Health(ctx, request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unauthenticated call: %v", err)
	}
	authed := metadata.AppendToOutgoingContext(ctx, "x-gotth-service-token", "12345678901234567890123456789012")
	health, err := client.Health(authed, request)
	if err != nil {
		t.Fatal(err)
	}
	if health.GetState() != extensionsv1.HealthState_HEALTH_STATE_READY {
		t.Fatalf("health: %+v", health)
	}
}
