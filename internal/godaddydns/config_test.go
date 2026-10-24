package godaddydns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	extensioncore "github.com/gotthboard/gotth-extensions/pkg/extensions"
)

func validConfigJSON() []byte {
	return []byte(`{"schema":"gotth.extension.godaddy-dns.config.v1","environment":"ote","zones":["example.com"],"record_types":["A","MX","SRV","TXT"],"timeout_seconds":15}`)
}

func TestDecodeConfig(t *testing.T) {
	t.Parallel()
	c, err := decodeConfig(validConfigJSON())
	if err != nil {
		t.Fatal(err)
	}
	if !c.AllowsZone("example.com") || c.AllowsZone("other.example") || !c.AllowsType("MX") || !c.AllowsType("SRV") {
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

func TestConfigurationMetadataMatchesMutableTypes(t *testing.T) {
	data, err := os.ReadFile("../../configuration-metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Fields []struct {
			Name   string   `json:"name"`
			Values []string `json:"values"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	var configured []string
	for _, field := range metadata.Fields {
		if field.Name == "record_types" {
			configured = field.Values
		}
	}
	compiled := make([]string, 0, len(mutableTypes))
	for recordType := range mutableTypes {
		compiled = append(compiled, recordType)
	}
	sort.Strings(compiled)
	if strings.Join(configured, ",") != strings.Join(compiled, ",") {
		t.Fatalf("metadata types=%v compiled types=%v", configured, compiled)
	}
}

func TestDecodeBindingCapabilities(t *testing.T) {
	t.Parallel()
	digest := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	raw, _ := json.Marshal(Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: digest, GrantSHA256: digest, SessionSHA256: digest, Interfaces: []extensioncore.InterfaceGrant{{Name: Interface, Major: 1, Minor: 0}}, Capabilities: []string{"dns.records.observe"}})
	b, err := decodeBinding(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !b.AllowsCapability("dns.records.observe") || b.AllowsCapability("dns.records.delete") {
		t.Fatal("capability subset ignored")
	}
	raw = []byte(`{"schema":"gotth.extension.runtime-binding.v1","instance_id":"11111111-1111-1111-1111-111111111111","manifest_sha256":"` + digest + `","grant_sha256":"` + digest + `","session_sha256":"` + digest + `","interfaces":[{"name":"gotth.dns.records","major":1,"minor":0}],"capabilities":["dns.records.replace","dns.records.observe"]}`)
	if _, err := decodeBinding(raw); err == nil {
		t.Fatal("unsorted capabilities accepted")
	}
	for name, interfaces := range map[string][]extensioncore.InterfaceGrant{
		"missing":     nil,
		"wrong name":  {{Name: "gotth.dns.other", Major: 1, Minor: 0}},
		"wrong major": {{Name: Interface, Major: 2, Minor: 0}},
		"wrong minor": {{Name: Interface, Major: 1, Minor: 1}},
		"duplicate":   {{Name: Interface, Major: 1, Minor: 0}, {Name: Interface, Major: 1, Minor: 0}},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: digest, GrantSHA256: digest, SessionSHA256: digest, Interfaces: interfaces, Capabilities: []string{"dns.records.observe"}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeBinding(raw); err == nil {
				t.Fatal("invalid negotiated interface accepted")
			}
		})
	}
	for name, interfacesJSON := range map[string]string{
		"duplicate nested name": `[{"name":"gotth.dns.other","name":"gotth.dns.records","major":1,"minor":0}]`,
		"unknown nested field":  `[{"name":"gotth.dns.records","major":1,"minor":0,"extra":false}]`,
	} {
		t.Run(name, func(t *testing.T) {
			raw := []byte(`{"schema":"gotth.extension.runtime-binding.v1","instance_id":"11111111-1111-1111-1111-111111111111","manifest_sha256":"` + digest + `","grant_sha256":"` + digest + `","session_sha256":"` + digest + `","interfaces":` + interfacesJSON + `,"capabilities":["dns.records.observe"]}`)
			if _, err := decodeBinding(raw); err == nil {
				t.Fatal("ambiguous nested interface binding accepted")
			}
		})
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
	binding, _ := json.Marshal(Binding{Schema: "gotth.extension.runtime-binding.v1", InstanceID: "11111111-1111-1111-1111-111111111111", ManifestSHA256: manifest, GrantSHA256: digest, SessionSHA256: digest, Interfaces: []extensioncore.InterfaceGrant{{Name: Interface, Major: 1, Minor: 0}}, Capabilities: []string{"dns.records.observe"}})
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
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte(strings.Repeat("a", 31)+"é"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRuntimeFromEnv(); err == nil {
		t.Fatal("non-ASCII service token accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("12345678901234567890123456789012"), 0o600); err != nil {
		t.Fatal(err)
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

func TestServiceTokenGrammarIsPrintableASCII(t *testing.T) {
	if !validServiceToken(strings.Repeat("!", 32)) || !validServiceToken(strings.Repeat("~", 32)) {
		t.Fatal("printable ASCII boundary rejected")
	}
	for _, token := range []string{strings.Repeat("a", 31), strings.Repeat("a", 257), strings.Repeat("a", 31) + " ", strings.Repeat("a", 31) + "\x7f", strings.Repeat("a", 31) + "é"} {
		if validServiceToken(token) {
			t.Fatalf("invalid token accepted: %q", token)
		}
	}
}

func TestRequestUUIDPolicyPreservesFoundationInstanceGrammar(t *testing.T) {
	if !validInstanceID("11111111-1111-1111-1111-111111111111") {
		t.Fatal("foundation lexical instance ID was narrowed")
	}
	valid := []string{
		"11111111-1111-1111-8111-111111111111",
		"22222222-2222-2222-9222-222222222222",
		"33333333-3333-3333-a333-333333333333",
		"44444444-4444-4444-b444-444444444444",
		"55555555-5555-5555-8555-555555555555",
	}
	for _, value := range valid {
		if !validRequestUUID(value) {
			t.Fatalf("RFC 4122 UUID rejected: %q", value)
		}
	}
	invalid := []string{
		"00000000-0000-0000-8000-000000000000",
		"66666666-6666-6666-8666-666666666666",
		"11111111-1111-1111-7111-111111111111",
		"11111111-1111-1111-c111-111111111111",
		"11111111-1111-1111-8111-11111111111A",
	}
	for _, value := range invalid {
		if validRequestUUID(value) {
			t.Fatalf("non-RFC 4122 request UUID accepted: %q", value)
		}
	}
}
