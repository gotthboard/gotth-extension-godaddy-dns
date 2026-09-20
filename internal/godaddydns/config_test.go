package godaddydns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func validConfigJSON() []byte {
	return []byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["A","MX","TXT"],"timeout_seconds":15}`)
}

func TestDecodeConfig(t *testing.T) {
	t.Parallel()
	c, err := decodeConfig(validConfigJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !c.AllowsZone("example.com") || c.AllowsZone("other.example") || !c.AllowsType("MX") {
		t.Fatal("allowlists not enforced")
	}
	bad := [][]byte{
		[]byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["A"],"timeout_seconds":15}`),
		[]byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"custom","zones":["example.com"],"record_types":["A"],"timeout_seconds":15}`),
		[]byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["EXAMPLE.com"],"record_types":["A"],"timeout_seconds":15}`),
		[]byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["NS"],"timeout_seconds":15}`),
		[]byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["TXT","A"],"timeout_seconds":15}`),
	}
	for i, raw := range bad {
		if _, err := decodeConfig(raw); err == nil {
			t.Fatalf("bad config %d accepted", i)
		}
	}
}

func TestDecodeBindingCapabilities(t *testing.T) {
	t.Parallel()
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	raw, _ := json.Marshal(Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: digest, GrantSHA256: digest, SessionSHA256: digest, Capabilities: []string{"dns.records.observe"}})
	b, err := decodeBinding(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !b.AllowsCapability("dns.records.observe") || b.AllowsCapability("dns.records.delete") {
		t.Fatal("capability subset ignored")
	}
	raw = []byte(`{"schema":"gotth.extension.runtime-binding.v1","instance_id":"11111111-1111-1111-1111-111111111111","manifest_sha256":"` + digest + `","grant_sha256":"` + digest + `","session_sha256":"` + digest + `","capabilities":["dns.records.replace","dns.records.observe"]}`)
	if _, err := decodeBinding(raw); err == nil {
		t.Fatal("unsorted capabilities accepted")
	}
}

func TestLoadRuntimeFromPrivateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	secretDir := filepath.Join(dir, "secrets")
	socketDir := filepath.Join(dir, "run")
	if err := os.Mkdir(secretDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(socketDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := ManifestDigest("dev")
	if err != nil {
		t.Fatal(err)
	}
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	binding, _ := json.Marshal(Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: manifest, GrantSHA256: digest, SessionSHA256: digest, Capabilities: []string{"dns.records.observe"}})
	files := map[string][]byte{"config": validConfigJSON(), "binding": binding, "token": []byte("12345678901234567890123456789012"), filepath.Join("secrets", SecretSlot): []byte("0123456789abcdef0123456789abcdef")}
	for name, data := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOTTH_EXTENSION_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("GOTTH_EXTENSION_BINDING_FILE", filepath.Join(dir, "binding"))
	t.Setenv("GOTTH_EXTENSION_SECRET_DIR", secretDir)
	t.Setenv("GOTTH_EXTENSION_SERVICE_TOKEN_FILE", filepath.Join(dir, "token"))
	t.Setenv("GOTTH_EXTENSION_SOCKET", filepath.Join(socketDir, "provider.sock"))
	runtime, err := LoadRuntimeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(runtime.PAT)
	if string(runtime.PAT) != "0123456789abcdef0123456789abcdef" {
		t.Fatal("PAT load mismatch")
	}
	if err := os.Chmod(filepath.Join(dir, "token"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeFromEnv(); err == nil {
		t.Fatal("public token file accepted")
	}
}

func TestReadPrivateChildRejectsSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := readPrivateChild(dir, "link", 1024); err == nil {
		t.Fatal("secret symlink accepted")
	}
	if got, err := readPrivateChild(dir, "target", 1024); err != nil || string(got) != "secret" {
		t.Fatalf("regular child rejected: %q %v", got, err)
	}
}
