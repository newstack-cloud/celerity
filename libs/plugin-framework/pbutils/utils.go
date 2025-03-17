package pbutils

import "google.golang.org/protobuf/types/known/wrapperspb"

// IntPtrFromPBWrapper converts a protobuf wrapperspb.Int64Value to a *int.
func IntPtrFromPBWrapper(wrapper *wrapperspb.Int64Value) *int {
	valuePtr := (*int)(nil)
	if wrapper != nil {
		intVal := int(wrapper.Value)
		valuePtr = &intVal
	}
	return valuePtr
}

// IntPtrToPBWrapper converts a *int to a protobuf wrapperspb.Int64Value.
func IntPtrToPBWrapper(value *int) *wrapperspb.Int64Value {
	if value == nil {
		return nil
	}
	return &wrapperspb.Int64Value{Value: int64(*value)}
}

// DoublePtrFromPBWrapper converts a protobuf wrapperspb.DoubleValue to a *float64.
func DoublePtrFromPBWrapper(wrapper *wrapperspb.DoubleValue) *float64 {
	valuePtr := (*float64)(nil)
	if wrapper != nil {
		floatVal := float64(wrapper.Value)
		valuePtr = &floatVal
	}
	return valuePtr
}

// DoublePtrToPBWrapper converts a *float64 to a protobuf wrapperspb.DoubleValue.
func DoublePtrToPBWrapper(value *float64) *wrapperspb.DoubleValue {
	if value == nil {
		return nil
	}
	return &wrapperspb.DoubleValue{Value: *value}
}
