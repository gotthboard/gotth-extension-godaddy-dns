package godaddydns

import (
	"strings"
	"testing"

	dnsv1 "github.com/gotthboard/gotth-extension-godaddy-dns/proto/gotth/dns/provider/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestDNSWireDescriptorHasNoGenericOrSecretFields(t *testing.T) {
	forbidden := []string{"secret", "token", "pat", "header", "url", "body", "metadata", "payload", "bytes"}
	var scan func(protoreflect.MessageDescriptors)
	scan = func(messages protoreflect.MessageDescriptors) {
		for i := 0; i < messages.Len(); i++ {
			message := messages.Get(i)
			fields := message.Fields()
			for j := 0; j < fields.Len(); j++ {
				field := fields.Get(j)
				name := strings.ToLower(string(field.Name()))
				if field.IsMap() || field.Kind() == protoreflect.BytesKind {
					t.Errorf("generic field %s.%s", message.FullName(), field.Name())
				}
				for _, word := range forbidden {
					if strings.Contains(name, word) {
						t.Errorf("forbidden field %s.%s", message.FullName(), field.Name())
					}
				}
			}
			scan(message.Messages())
		}
	}
	scan(dnsv1.File_proto_gotth_dns_provider_v1_dns_proto.Messages())
}
