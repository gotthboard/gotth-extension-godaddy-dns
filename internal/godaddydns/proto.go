package godaddydns

import (
	"errors"

	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
)

var typeToProto = map[string]dnsv1.RecordType{
	"A": dnsv1.RecordType_RECORD_TYPE_A, "AAAA": dnsv1.RecordType_RECORD_TYPE_AAAA,
	"CNAME": dnsv1.RecordType_RECORD_TYPE_CNAME, "MX": dnsv1.RecordType_RECORD_TYPE_MX,
	"TXT": dnsv1.RecordType_RECORD_TYPE_TXT, "NS": dnsv1.RecordType_RECORD_TYPE_NS,
	"SRV": dnsv1.RecordType_RECORD_TYPE_SRV, "SOA": dnsv1.RecordType_RECORD_TYPE_SOA,
	"CAA": dnsv1.RecordType_RECORD_TYPE_CAA,
}
var protoToType = map[dnsv1.RecordType]string{
	dnsv1.RecordType_RECORD_TYPE_A: "A", dnsv1.RecordType_RECORD_TYPE_AAAA: "AAAA",
	dnsv1.RecordType_RECORD_TYPE_CNAME: "CNAME", dnsv1.RecordType_RECORD_TYPE_MX: "MX",
	dnsv1.RecordType_RECORD_TYPE_TXT: "TXT", dnsv1.RecordType_RECORD_TYPE_NS: "NS",
	dnsv1.RecordType_RECORD_TYPE_SRV: "SRV", dnsv1.RecordType_RECORD_TYPE_SOA: "SOA",
	dnsv1.RecordType_RECORD_TYPE_CAA: "CAA",
}

func recordFromProto(in *dnsv1.Record, observed bool) (apiRecord, error) {
	if in == nil {
		return apiRecord{}, errors.New("record required")
	}
	typ, ok := protoToType[in.GetType()]
	if !ok {
		return apiRecord{}, errors.New("record type required")
	}
	r := apiRecord{RecordID: in.GetRecordId(), Name: in.GetName(), Type: typ, Data: in.GetData(), TTL: in.GetTtl(), Priority: cloneUint32(in.Priority), Service: in.GetService(), Protocol: in.GetProtocol(), Weight: cloneUint32(in.Weight), Port: cloneUint32(in.Port), Flag: cloneUint32(in.Flag), Tag: in.GetTag()}
	return canonicalRecord(r, observed)
}

func recordProto(r apiRecord) *dnsv1.Record {
	return &dnsv1.Record{RecordId: r.RecordID, Name: r.Name, Type: typeToProto[r.Type], Data: r.Data, Ttl: r.TTL, Priority: cloneUint32(r.Priority), Service: r.Service, Protocol: r.Protocol, Weight: cloneUint32(r.Weight), Port: cloneUint32(r.Port), Flag: cloneUint32(r.Flag), Tag: r.Tag}
}

func snapshotProto(s snapshot) *dnsv1.Snapshot {
	out := &dnsv1.Snapshot{Zone: s.Zone, Sha256: s.Digest, Records: make([]*dnsv1.Record, len(s.Records))}
	for i, r := range s.Records {
		out.Records[i] = recordProto(r)
	}
	return out
}

func cloneUint32(in *uint32) *uint32 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
