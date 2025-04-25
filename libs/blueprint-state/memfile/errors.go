package memfile

import (
	"fmt"
)

// Error is a custom error type that provides errors specific to
// the memfile (in-memory, persisted to file) implementation of the state.Container interface.
type Error struct {
	ReasonCode ErrorReasonCode
	Err        error
}

func (e *Error) Error() string {
	return fmt.Sprintf("memfile state error (%s): %s", e.ReasonCode, e.Err)
}

// ErrorReasonCode is an enum of possible error reasons that can be returned by the memfile implementation.
type ErrorReasonCode string

const (
	// ErrorReasonCodeMalformedStateFile is the error code that is used when
	// a state file is malformed,
	// this could be due to a file being corrupted or a mismatch between
	// the index and the actual state file.
	ErrorReasonCodeMalformedStateFile ErrorReasonCode = "malformed_state_file"

	// ErrorReasonCodeMalformedState is the error code that is used when
	// the in-memory state is malformed, usually when the instance associated
	// with a resource or link no longer exists but the resource or link
	// still exists.
	ErrorReasonCodeMalformedState ErrorReasonCode = "malformed_state"

	// ErrorReasonCodeMaxEventPartitionSizeExceeded is the error code that is used when
	// the maximum event partition size is exceeded when trying to save an event.
	ErrorReasonCodeMaxEventPartitionSizeExceeded ErrorReasonCode = "max_event_partition_size_exceeded"
)

func errMalformedState(message string) error {
	return &Error{
		ReasonCode: ErrorReasonCodeMalformedState,
		Err:        fmt.Errorf("malformed state: %s", message),
	}
}

func errMalformedStateFile(message string) error {
	return &Error{
		ReasonCode: ErrorReasonCodeMalformedStateFile,
		Err:        fmt.Errorf("malformed state file: %s", message),
	}
}

func errMaxEventPartitionSizeExceeded(
	partition string,
	maxEventPartitionSize int64,
) error {
	return &Error{
		ReasonCode: ErrorReasonCodeMaxEventPartitionSizeExceeded,
		Err: fmt.Errorf(
			"maximum event partition size (%d bytes) exceeded for %q",
			maxEventPartitionSize,
			partition,
		),
	}
}
