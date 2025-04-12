package utils

import "reflect"

// IsAnyNil checks if the provided value is nil or a nil pointer.
func IsAnyNil(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	k := v.Kind()
	switch k {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice,
		reflect.UnsafePointer, reflect.Interface:
		return v.IsNil()
	}

	return false
}
