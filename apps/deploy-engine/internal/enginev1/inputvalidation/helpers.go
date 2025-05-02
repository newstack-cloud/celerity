package inputvalidation

import (
	"reflect"
	"strings"
)

// JSONTagNameFunc is a function that returns the JSON tag name for a field
// to be used with validate.RegisterTagNameFunc for validators created with
// the go-playground/validator package.
func JSONTagNameFunc(fld reflect.StructField) string {
	name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
	// skip if tag key says it should be ignored
	if name == "-" {
		return ""
	}
	return name
}
