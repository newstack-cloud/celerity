package resolve

import "fmt"

// InvalidLocationMetadataError is an error that indicates that the
// location metadata provided in a request payload that involves resolving
// a blueprint document is invalid.
//
// Example to check for this error type:
//
//	var targetErr *resolve.InvalidLocationMetadataError
//	if errors.As(err, &targetErr) {
//		// Handle the error
//	}
type InvalidLocationMetadataError struct {
	Reason string
}

func (e *InvalidLocationMetadataError) Error() string {
	return fmt.Sprintf(
		"invalid location metadata: %s",
		e.Reason,
	)
}
