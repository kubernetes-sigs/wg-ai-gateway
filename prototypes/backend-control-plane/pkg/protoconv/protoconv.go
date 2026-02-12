// Borrowed with love from Istio
package protoconv

import (
	"fmt"

	udpa "github.com/cncf/xds/go/udpa/type/v1"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/klog/v2"
)

// MessageToAnyWithError converts from proto message to proto Any
func MessageToAnyWithError(msg proto.Message) (*anypb.Any, error) {
	b, err := marshal(msg)
	if err != nil {
		return nil, err
	}
	return &anypb.Any{
		// nolint: staticcheck
		TypeUrl: "type.googleapis.com/" + string(msg.ProtoReflect().Descriptor().FullName()),
		Value:   b,
	}, nil
}

func marshal(msg proto.Message) ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(msg)
}

// MessageToAny converts from proto message to proto Any
func MessageToAny(msg proto.Message) *anypb.Any {
	out, err := MessageToAnyWithError(msg)
	if err != nil {
		klog.Error(fmt.Printf("error marshaling Any %s: %v", prototext.Format(msg), err))
		return nil
	}
	return out
}

func TypedStructWithFields(typeURL string, fields map[string]interface{}) *anypb.Any {
	value, err := structpb.NewStruct(fields)
	if err != nil {
		klog.Error(fmt.Sprintf("error marshaling struct %s: %v", typeURL, err))
	}
	return MessageToAny(&udpa.TypedStruct{
		TypeUrl: typeURL,
		Value:   value,
	})
}

func SilentlyUnmarshalAny[T any](a *anypb.Any) *T {
	res, err := UnmarshalAny[T](a)
	if err != nil {
		return nil
	}
	return res
}

func UnmarshalAny[T any](a *anypb.Any) (*T, error) {
	dst := any(new(T)).(proto.Message)
	if err := a.UnmarshalTo(dst); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to %T: %v", dst, err)
	}
	return any(dst).(*T), nil
}

// Equals checks if two message are equal.
func Equals[T proto.Message](a T, b T) bool {
	return proto.Equal(a, b)
}
