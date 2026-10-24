package godaddydns

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	extensionsv1 "github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestPublishedControlDescriptorBinding(t *testing.T) {
	file := protodesc.ToFileDescriptorProto(extensionsv1.File_proto_gotth_extensions_v1_control_proto)
	if file.GetName() != "proto/gotth/extensions/v1/control.proto" || file.GetPackage() != "gotth.extensions.v1" {
		t.Fatalf("unexpected control descriptor identity: name=%q package=%q", file.GetName(), file.GetPackage())
	}
	const goPackage = "github.com/gotthboard/gotth-sdk/gen/go/gotth/extensions/v1;extensionsv1"
	if file.GetOptions().GetGoPackage() != goPackage {
		t.Fatalf("unexpected control Go package: %q", file.GetOptions().GetGoPackage())
	}
	set := &descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	const wantDescriptorSHA256 = "62a549b29a030f0decb46b2b1c2f382bbbc75430ba0b3d7c729bd21528b505cf"
	if got := hex.EncodeToString(digest[:]); got != wantDescriptorSHA256 {
		t.Fatalf("control descriptor SHA-256 = %s, want %s", got, wantDescriptorSHA256)
	}
	if extensionsv1.ExtensionControl_Handshake_FullMethodName != "/gotth.extensions.v1.ExtensionControl/Handshake" || extensionsv1.ExtensionControl_Health_FullMethodName != "/gotth.extensions.v1.ExtensionControl/Health" {
		t.Fatal("published control RPC paths drifted")
	}
}
