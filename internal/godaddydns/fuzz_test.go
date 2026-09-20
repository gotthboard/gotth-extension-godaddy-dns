package godaddydns

import "testing"

func FuzzStrictProviderJSON(f *testing.F) {
	f.Add([]byte(`{"items":[],"totalItems":0,"totalPages":1,"links":[{"rel":"self","href":"https://api.godaddy.invalid/page"}]}`))
	f.Add([]byte(`{"items":[],"items":[]}`))
	f.Add([]byte{0xff, 0x00, '{'})
	f.Fuzz(func(t *testing.T, data []byte) { var page pageResponse; _ = strictDecode(data, &page) })
}

func FuzzConfigurationDecoder(f *testing.F) {
	f.Add(validConfigJSON())
	f.Add([]byte(`{"schema":"wrong"}`))
	f.Add([]byte{0xff, '[', ']'})
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = decodeConfig(data) })
}

func FuzzRecordCanonicalization(f *testing.F) {
	f.Add("mail", "A", "192.0.2.1", uint32(600))
	f.Add("_dmarc", "TXT", "v=DMARC1", uint32(3600))
	f.Fuzz(func(t *testing.T, name, typ, data string, ttl uint32) {
		_, _ = canonicalRecord(apiRecord{Name: name, Type: typ, Data: data, TTL: ttl}, false)
	})
}
