package godaddydns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	extensioncore "github.com/gotthboard/gotth-extensions/pkg/extensions"
	"golang.org/x/sys/unix"
)

const (
	ConfigSchema = "gotth.extension.godaddy-dns.config.v1"
	ExtensionID  = "gotth.dns.provider.godaddy"
	Interface    = "gotth.dns.records"
	SecretSlot   = "godaddy.pat"
	maxFileBytes = 64 << 10
)

var (
	capabilities = []string{"dns.records.create", "dns.records.delete", "dns.records.observe", "dns.records.replace"}
	BuildVersion = "dev"
	mutableTypes = map[string]bool{"A": true, "AAAA": true, "CAA": true, "CNAME": true, "MX": true, "SRV": true, "TXT": true}
)

type Config struct {
	Schema         string   `json:"schema"`
	Environment    string   `json:"environment"`
	Zones          []string `json:"zones"`
	RecordTypes    []string `json:"record_types"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type Binding struct {
	Schema         string                         `json:"schema"`
	InstanceID     string                         `json:"instance_id"`
	ManifestSHA256 string                         `json:"manifest_sha256"`
	GrantSHA256    string                         `json:"grant_sha256"`
	SessionSHA256  string                         `json:"session_sha256"`
	Interfaces     []extensioncore.InterfaceGrant `json:"interfaces"`
	Capabilities   []string                       `json:"capabilities"`
}

type RuntimeConfig struct {
	Config       Config
	Binding      Binding
	ServiceToken string
	PAT          []byte
	SocketPath   string
	Version      string
}

func LoadRuntimeFromEnv() (RuntimeConfig, error) {
	configPath := os.Getenv("GOTTH_EXTENSION_CONFIG_FILE")
	bindingPath := os.Getenv("GOTTH_EXTENSION_BINDING_FILE")
	secretDir := os.Getenv("GOTTH_EXTENSION_SECRET_DIR")
	tokenPath := os.Getenv("GOTTH_EXTENSION_SERVICE_TOKEN_FILE")
	socketPath := os.Getenv("GOTTH_EXTENSION_SOCKET")
	if configPath == "" || bindingPath == "" || secretDir == "" || tokenPath == "" || socketPath == "" || !validVersion(BuildVersion) {
		return RuntimeConfig{}, errors.New("complete extension runtime configuration required")
	}
	configData, err := readPrivateFile(configPath, maxFileBytes)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read configuration: %w", err)
	}
	bindingData, err := readPrivateFile(bindingPath, maxFileBytes)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read binding: %w", err)
	}
	tokenData, err := readPrivateFile(tokenPath, 8<<10)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("read service token: %w", err)
	}
	pat, err := readPrivateChild(secretDir, SecretSlot, 8<<10)
	if err != nil {
		return RuntimeConfig{}, errors.New("valid GoDaddy PAT required")
	}
	if !bytes.Equal(bytes.TrimSpace(pat), pat) || !validPAT(string(pat)) {
		clear(pat)
		return RuntimeConfig{}, errors.New("valid GoDaddy PAT required")
	}
	config, err := decodeConfig(configData)
	if err != nil {
		clear(pat)
		return RuntimeConfig{}, err
	}
	binding, err := decodeBinding(bindingData)
	if err != nil {
		clear(pat)
		return RuntimeConfig{}, err
	}
	manifestDigest, err := ManifestDigest(BuildVersion)
	if err != nil || manifestDigest != binding.ManifestSHA256 {
		clear(pat)
		return RuntimeConfig{}, errors.New("artifact manifest binding mismatch")
	}
	token := string(tokenData)
	if !validServiceToken(token) {
		clear(pat)
		return RuntimeConfig{}, errors.New("valid service token required")
	}
	if err := validateSocketPath(socketPath); err != nil {
		clear(pat)
		return RuntimeConfig{}, err
	}
	return RuntimeConfig{Config: config, Binding: binding, ServiceToken: token, PAT: pat, SocketPath: socketPath, Version: BuildVersion}, nil
}

func decodeConfig(data []byte) (Config, error) {
	fields, err := decodeUniqueObject(data)
	if err != nil || len(fields) != 5 {
		return Config{}, errors.New("invalid extension configuration")
	}
	var c Config
	if decodeField(fields, "schema", &c.Schema) != nil || c.Schema != ConfigSchema {
		return Config{}, errors.New("invalid configuration schema")
	}
	if decodeField(fields, "environment", &c.Environment) != nil || c.Environment != "production" && c.Environment != "ote" {
		return Config{}, errors.New("invalid GoDaddy environment")
	}
	if decodeField(fields, "zones", &c.Zones) != nil || len(c.Zones) < 1 || len(c.Zones) > 32 {
		return Config{}, errors.New("bounded zone allowlist required")
	}
	if decodeField(fields, "record_types", &c.RecordTypes) != nil || len(c.RecordTypes) < 1 || len(c.RecordTypes) > len(mutableTypes) {
		return Config{}, errors.New("bounded record type allowlist required")
	}
	if decodeField(fields, "timeout_seconds", &c.TimeoutSeconds) != nil || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 30 {
		return Config{}, errors.New("timeout must be between 1 and 30 seconds")
	}
	if !sort.StringsAreSorted(c.Zones) || !sort.StringsAreSorted(c.RecordTypes) {
		return Config{}, errors.New("configuration allowlists must be sorted")
	}
	for i, zone := range c.Zones {
		if !validZone(zone) || i > 0 && zone == c.Zones[i-1] {
			return Config{}, errors.New("valid unique zones required")
		}
	}
	for i, typ := range c.RecordTypes {
		if !mutableTypes[typ] || i > 0 && typ == c.RecordTypes[i-1] {
			return Config{}, errors.New("valid unique mutable record types required")
		}
	}
	return c, nil
}

// Complexity: time O(n + c + i), Omega(1), tight Theta(n + c + i) on a valid
// binding; auxiliary space O(n + c + i), Omega(1), tight Theta(n + c + i) on
// a valid binding; variables: n input bytes, c capabilities, i interfaces.
func decodeBinding(data []byte) (Binding, error) {
	fields, err := decodeUniqueObject(data)
	if err != nil || len(fields) != 7 {
		return Binding{}, errors.New("invalid extension binding")
	}
	var b Binding
	if decodeField(fields, "schema", &b.Schema) != nil || b.Schema != "gotth.extension.runtime-binding.v1" {
		return Binding{}, errors.New("invalid extension binding schema")
	}
	if decodeField(fields, "instance_id", &b.InstanceID) != nil || !validInstanceID(b.InstanceID) {
		return Binding{}, errors.New("valid instance binding required")
	}
	for name, target := range map[string]*string{"manifest_sha256": &b.ManifestSHA256, "grant_sha256": &b.GrantSHA256, "session_sha256": &b.SessionSHA256} {
		if decodeField(fields, name, target) != nil || !validHex(*target, 64) {
			return Binding{}, errors.New("valid digest binding required")
		}
	}
	if decodeField(fields, "capabilities", &b.Capabilities) != nil || len(b.Capabilities) < 1 || len(b.Capabilities) > len(capabilities) || !sort.StringsAreSorted(b.Capabilities) {
		return Binding{}, errors.New("valid capability binding required")
	}
	for i, capability := range b.Capabilities {
		j := sort.SearchStrings(capabilities, capability)
		if j == len(capabilities) || capabilities[j] != capability || i > 0 && capability == b.Capabilities[i-1] {
			return Binding{}, errors.New("valid capability binding required")
		}
	}
	interfacesJSON, ok := fields["interfaces"]
	if !ok || strictDecode(interfacesJSON, &b.Interfaces) != nil || len(b.Interfaces) != 1 || b.Interfaces[0] != (extensioncore.InterfaceGrant{Name: Interface, Major: 1, Minor: 0}) {
		return Binding{}, errors.New("exact negotiated DNS interface required")
	}
	return b, nil
}

func decodeUniqueObject(data []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, errors.New("JSON object required")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, err := d.Token()
		name, ok := t.(string)
		if err != nil || !ok {
			return nil, errors.New("invalid JSON object")
		}
		if _, ok := fields[name]; ok {
			return nil, errors.New("duplicate JSON field")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		fields[name] = raw
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') {
		return nil, errors.New("invalid JSON object")
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing JSON input")
	}
	return fields, nil
}

func decodeField(fields map[string]json.RawMessage, name string, target any) error {
	raw, ok := fields[name]
	if !ok {
		return errors.New("missing field")
	}
	return json.Unmarshal(raw, target)
}

func readPrivateFile(path string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("canonical absolute private file path required")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || !ownedByEUID(info) {
		return nil, errors.New("private owned regular file required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0o077 != 0 || !ownedByEUID(opened) {
		return nil, errors.New("private regular file changed during open")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("bounded private file required")
	}
	return data, nil
}

func ownedByEUID(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid())
}

func validatePrivateDir(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("canonical absolute private directory path required")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ownedByEUID(info) {
		return errors.New("private owned directory required")
	}
	dir, err := os.Open(path)
	if err != nil {
		return errors.New("private owned directory required")
	}
	defer dir.Close()
	opened, err := dir.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.IsDir() || opened.Mode().Perm()&0o077 != 0 || !ownedByEUID(opened) {
		return errors.New("private directory changed during open")
	}
	return nil
}

func readPrivateChild(dirPath, name string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(dirPath) || filepath.Clean(dirPath) != dirPath {
		return nil, errors.New("canonical absolute private directory path required")
	}
	if name == "" || filepath.Base(name) != name {
		return nil, errors.New("bounded child name required")
	}
	info, err := os.Lstat(dirPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 || !ownedByEUID(info) {
		return nil, errors.New("private owned directory required")
	}
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	opened, err := dir.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.IsDir() || opened.Mode().Perm()&0o077 != 0 || !ownedByEUID(opened) {
		return nil, errors.New("private directory changed during open")
	}
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("private child file required")
	}
	f := os.NewFile(uintptr(fd), name)
	if f == nil {
		_ = unix.Close(fd)
		return nil, errors.New("private child file required")
	}
	defer f.Close()
	child, err := f.Stat()
	if err != nil || !child.Mode().IsRegular() || child.Mode().Perm()&0o077 != 0 || !ownedByEUID(child) {
		return nil, errors.New("private owned regular child required")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("bounded private file required")
	}
	return data, nil
}

func validateSocketPath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 100 {
		return errors.New("bounded absolute Unix socket path required")
	}
	if err := validatePrivateDir(filepath.Dir(path)); err != nil {
		return errors.New("private Unix socket directory required")
	}
	if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("Unix socket path must not exist")
	}
	return nil
}

// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1), because both accepted strings have fixed length.
func validVersion(v string) bool { return v == "dev" || v == "1.0.0" }

// Complexity: time O(n), Omega(1), tight Theta(n) when all runes are valid;
// auxiliary space O(1), Omega(1), tight Theta(1); variable: n is len(v).
func validPAT(v string) bool {
	if len(v) < 16 || len(v) > 4096 || !utf8.ValidString(v) {
		return false
	}
	for _, r := range v {
		if r <= 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// Complexity: time O(n), Omega(1), tight Theta(n) when all bytes are valid;
// auxiliary space O(1), Omega(1), tight Theta(1); variable: n is len(v).
func validServiceToken(v string) bool {
	if len(v) < 32 || len(v) > 256 {
		return false
	}
	for i := range len(v) {
		if v[i] < 0x21 || v[i] > 0x7e {
			return false
		}
	}
	return true
}

func validHex(v string, n int) bool {
	if len(v) != n {
		return false
	}
	for _, r := range v {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1), because input and allocation are bounded by 36
// and 32 bytes respectively.
func validInstanceID(v string) bool {
	return len(v) == 36 && v[8] == '-' && v[13] == '-' && v[18] == '-' && v[23] == '-' && validHex(strings.ReplaceAll(v, "-", ""), 32)
}

// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1), because request UUID text is fixed at 36 bytes.
func validRequestUUID(v string) bool {
	return validInstanceID(v) && v[14] >= '1' && v[14] <= '5' && strings.ContainsRune("89ab", rune(v[19]))
}

func validZone(v string) bool {
	if len(v) < 1 || len(v) > 253 || strings.HasSuffix(v, ".") || strings.ToLower(v) != v || strings.Contains(v, "..") {
		return false
	}
	for _, label := range strings.Split(v, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return strings.Contains(v, ".")
}

func (c Config) Timeout() time.Duration { return time.Duration(c.TimeoutSeconds) * time.Second }
func (c Config) AllowsZone(zone string) bool {
	i := sort.SearchStrings(c.Zones, zone)
	return i < len(c.Zones) && c.Zones[i] == zone
}
func (c Config) AllowsType(typ string) bool {
	i := sort.SearchStrings(c.RecordTypes, typ)
	return i < len(c.RecordTypes) && c.RecordTypes[i] == typ
}
func (b Binding) AllowsCapability(capability string) bool {
	i := sort.SearchStrings(b.Capabilities, capability)
	return i < len(b.Capabilities) && b.Capabilities[i] == capability
}

// Complexity: time O(n), Omega(1), with no tight single bound; auxiliary space
// Theta(1), where n is the number of negotiated interfaces.
func (b Binding) AllowsInterface(name string, major, minor uint32) bool {
	want := extensioncore.InterfaceGrant{Name: name, Major: major, Minor: minor}
	for _, item := range b.Interfaces {
		if item == want {
			return true
		}
	}
	return false
}
