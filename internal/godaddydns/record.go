package godaddydns

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"unicode/utf8"
)

type apiRecord struct {
	RecordID string  `json:"recordId,omitempty"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Data     string  `json:"data"`
	TTL      uint32  `json:"ttl"`
	Priority *uint32 `json:"priority,omitempty"`
	Service  string  `json:"service,omitempty"`
	Port     *uint32 `json:"port,omitempty"`
	Weight   *uint32 `json:"weight,omitempty"`
	Protocol string  `json:"protocol,omitempty"`
	Flag     *uint32 `json:"flag,omitempty"`
	Tag      string  `json:"tag,omitempty"`
}

type snapshot struct {
	Zone    string
	Digest  string
	Records []apiRecord
}

var observedTypes = map[string]bool{"A": true, "AAAA": true, "CAA": true, "CNAME": true, "MX": true, "NS": true, "SOA": true, "SRV": true, "TXT": true}

func canonicalRecord(in apiRecord, observed bool) (apiRecord, error) {
	r := in
	if !observedTypes[r.Type] || !validRecordName(r.Name) || r.TTL < 600 || r.TTL > 86400 || len(r.Data) < 1 || len(r.Data) > 512 || !validText(r.Data) {
		return apiRecord{}, errors.New("invalid DNS record")
	}
	if observed {
		if !validOpaqueID(r.RecordID) {
			return apiRecord{}, errors.New("invalid provider record ID")
		}
	} else if r.RecordID != "" || !mutableTypes[r.Type] {
		return apiRecord{}, errors.New("invalid mutable DNS record")
	}
	switch r.Type {
	case "A":
		addr, err := netip.ParseAddr(r.Data)
		if err != nil || !addr.Is4() || !noAux(r) {
			return apiRecord{}, errors.New("invalid A record")
		}
		r.Data = addr.String()
	case "AAAA":
		addr, err := netip.ParseAddr(r.Data)
		if err != nil || !addr.Is6() || !noAux(r) {
			return apiRecord{}, errors.New("invalid AAAA record")
		}
		r.Data = addr.String()
	case "CNAME":
		if r.Name == "@" || !validAbsoluteDNSName(r.Data) || !noAux(r) {
			return apiRecord{}, errors.New("invalid CNAME record")
		}
		r.Data = strings.ToLower(r.Data)
	case "MX":
		if !validAbsoluteDNSName(r.Data) || r.Priority == nil || *r.Priority > 65535 || r.Service != "" || r.Protocol != "" || r.Weight != nil || r.Port != nil || r.Flag != nil || r.Tag != "" {
			return apiRecord{}, errors.New("invalid MX record")
		}
		r.Data = strings.ToLower(r.Data)
	case "TXT":
		if !noAux(r) {
			return apiRecord{}, errors.New("invalid TXT record")
		}
	case "CAA":
		if r.Flag == nil || *r.Flag > 255 || !validCAATag(r.Tag) || r.Priority != nil || r.Service != "" || r.Protocol != "" || r.Weight != nil || r.Port != nil {
			return apiRecord{}, errors.New("invalid CAA record")
		}
		if !observed && *r.Flag != 0 && *r.Flag != 128 {
			return apiRecord{}, errors.New("invalid mutable CAA flags")
		}
		r.Tag = strings.ToLower(r.Tag)
	case "SRV":
		if !validService(r.Service) || !validProtocol(r.Protocol) || !validSRVOwner(r.Name, r.Service, r.Protocol) || (r.Data != "." && !validAbsoluteDNSName(r.Data)) || r.Priority == nil || r.Weight == nil || r.Port == nil || *r.Priority > 65535 || *r.Weight > 65535 || *r.Port > 65535 || r.Flag != nil || r.Tag != "" {
			return apiRecord{}, errors.New("invalid SRV record")
		}
		r.Data, r.Service, r.Protocol = strings.ToLower(r.Data), strings.ToLower(r.Service), strings.ToLower(r.Protocol)
	case "NS":
		if !observed || !validAbsoluteDNSName(r.Data) || !noAux(r) {
			return apiRecord{}, errors.New("invalid NS record")
		}
		r.Data = strings.ToLower(r.Data)
	case "SOA":
		if !observed || !noAux(r) {
			return apiRecord{}, errors.New("invalid SOA record")
		}
	}
	return r, nil
}

func noAux(r apiRecord) bool {
	return r.Priority == nil && r.Service == "" && r.Protocol == "" && r.Weight == nil && r.Port == nil && r.Flag == nil && r.Tag == ""
}

func validRecordName(v string) bool {
	if v == "@" {
		return true
	}
	if len(v) < 1 || len(v) > 255 || strings.ToLower(v) != v || strings.HasPrefix(v, ".") || strings.HasSuffix(v, ".") || strings.Contains(v, "..") {
		return false
	}
	for i, label := range strings.Split(v, ".") {
		if label == "*" {
			if i != 0 {
				return false
			}
			continue
		}
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return false
			}
		}
	}
	return true
}

func validAbsoluteDNSName(v string) bool {
	if len(v) < 2 || len(v) > 254 || !strings.HasSuffix(v, ".") || strings.ToLower(v) != v {
		return false
	}
	return validZone(strings.TrimSuffix(v, "."))
}

func validText(v string) bool {
	if !utf8.ValidString(v) {
		return false
	}
	for _, r := range v {
		if r == 0 || r == '\r' || r == '\n' || r < 0x20 && r != '\t' {
			return false
		}
	}
	return true
}
func validOpaqueID(v string) bool {
	if len(v) < 1 || len(v) > 256 {
		return false
	}
	for _, r := range v {
		if r < 0x21 || r > 0x7e || r == '/' || r == '?' || r == '#' {
			return false
		}
	}
	return true
}
func validCAATag(v string) bool {
	if len(v) < 1 || len(v) > 255 {
		return false
	}
	for _, c := range v {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func validService(v string) bool {
	if len(v) < 2 || len(v) > 63 || v[0] != '_' || v[1] == '-' || v[len(v)-1] == '-' || strings.ToLower(v) != v {
		return false
	}
	for _, character := range v[1:] {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-') {
			return false
		}
	}
	return true
}
func validProtocol(v string) bool { return v == "_tcp" || v == "_udp" || v == "_sctp" }

func validSRVOwner(name, service, protocol string) bool {
	prefix := service + "." + protocol
	if name == prefix {
		return true
	}
	suffix := strings.TrimPrefix(name, prefix+".")
	return strings.HasPrefix(name, prefix+".") && !strings.Contains(suffix, "*") && validRecordName(suffix)
}

func canonicalRecordBytes(r apiRecord, includeID bool) []byte {
	if !includeID {
		r.RecordID = ""
	}
	b, _ := json.Marshal(r)
	return b
}

func compareOptionalUint32(a, b *uint32) int {
	if a == nil {
		if b == nil {
			return 0
		}
		return -1
	}
	if b == nil {
		return 1
	}
	if *a < *b {
		return -1
	}
	if *a > *b {
		return 1
	}
	return 0
}

func compareString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func recordLess(a, b apiRecord) bool {
	if order := compareString(a.Type, b.Type); order != 0 {
		return order < 0
	}
	if order := compareString(a.Name, b.Name); order != 0 {
		return order < 0
	}
	if order := compareString(a.Data, b.Data); order != 0 {
		return order < 0
	}
	if order := compareOptionalUint32(a.Priority, b.Priority); order != 0 {
		return order < 0
	}
	if order := compareString(a.Service, b.Service); order != 0 {
		return order < 0
	}
	if order := compareString(a.Protocol, b.Protocol); order != 0 {
		return order < 0
	}
	if order := compareOptionalUint32(a.Weight, b.Weight); order != 0 {
		return order < 0
	}
	if order := compareOptionalUint32(a.Port, b.Port); order != 0 {
		return order < 0
	}
	if order := compareOptionalUint32(a.Flag, b.Flag); order != 0 {
		return order < 0
	}
	if a.Tag != b.Tag {
		return a.Tag < b.Tag
	}
	if a.TTL != b.TTL {
		return a.TTL < b.TTL
	}
	return a.RecordID < b.RecordID
}

func recordDigest(r apiRecord) string {
	sum := sha256.Sum256(canonicalRecordBytes(r, false))
	return hex.EncodeToString(sum[:])
}

func buildSnapshot(zone string, records []apiRecord) (snapshot, error) {
	canonical := make([]apiRecord, len(records))
	ids := make(map[string]struct{}, len(records))
	for i, r := range records {
		c, err := canonicalRecord(r, true)
		if err != nil {
			return snapshot{}, err
		}
		if _, ok := ids[c.RecordID]; ok {
			return snapshot{}, errors.New("duplicate provider record ID")
		}
		ids[c.RecordID] = struct{}{}
		canonical[i] = c
	}
	sort.Slice(canonical, func(i, j int) bool {
		return recordLess(canonical[i], canonical[j])
	})
	h := sha256.New()
	_, _ = h.Write([]byte("gotth.dns.snapshot.v1\x00"))
	_, _ = h.Write([]byte(zone))
	_, _ = h.Write([]byte{0})
	for _, r := range canonical {
		_, _ = h.Write(canonicalRecordBytes(r, true))
		_, _ = h.Write([]byte{'\n'})
	}
	return snapshot{Zone: zone, Digest: hex.EncodeToString(h.Sum(nil)), Records: canonical}, nil
}

func snapshotsEqual(a, b snapshot) bool {
	if a.Zone != b.Zone || a.Digest != b.Digest || len(a.Records) != len(b.Records) {
		return false
	}
	for i := range a.Records {
		if !bytes.Equal(canonicalRecordBytes(a.Records[i], true), canonicalRecordBytes(b.Records[i], true)) {
			return false
		}
	}
	return true
}

func findRecord(s snapshot, id string) (apiRecord, bool) {
	for _, r := range s.Records {
		if r.RecordID == id {
			return r, true
		}
	}
	return apiRecord{}, false
}
func logicalCount(s snapshot, desired apiRecord) int {
	n := 0
	want := canonicalRecordBytes(desired, false)
	for _, r := range s.Records {
		if bytes.Equal(canonicalRecordBytes(r, false), want) {
			n++
		}
	}
	return n
}
