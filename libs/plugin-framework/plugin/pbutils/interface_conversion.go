package pbutils

import (
	"encoding/json"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ConvertInterfaceToProtobuf converts an interface{} to a protobuf message any.
func ConvertInterfaceToProtobuf(value any) (*anypb.Any, error) {
	pbAnyValue := &anypb.Any{}
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	bytesValue := &wrapperspb.BytesValue{
		Value: bytes,
	}
	err = anypb.MarshalFrom(pbAnyValue, bytesValue, proto.MarshalOptions{})
	return pbAnyValue, err
}

// ConvertPBAnyToInterface converts a protobuf message any to an interface{}.
func ConvertPBAnyToInterface(pbAny *anypb.Any) (any, error) {
	var value any
	bytesValue := &wrapperspb.BytesValue{}
	err := anypb.UnmarshalTo(pbAny, bytesValue, proto.UnmarshalOptions{})
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytesValue.Value, &value)
	return value, err
}
