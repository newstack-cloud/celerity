package providerv1

import (
	"context"
	"fmt"
	"time"

	"github.com/two-hundred/celerity/libs/deploy-engine/plugin/providerserverv1"
)

// ContextFuncReturnValue is a function that takes a context,
// an argument and returns a value and an error.
type ContextFuncReturnValue[Arg any, Value any] func(context.Context, Arg) (Value, error)

// ContextFunc is a function that takes a context
// and an argument and returns an error.
type ContextFunc[Arg any] func(context.Context, Arg) error

// PluginError provides a custom error type to be used in the plugin system,
// this is converted to a logical error when returning to the deploy engine host
// as opposed to a protocol error.
type PluginError struct {
	ErrorCode providerserverv1.ErrorCode
	Message   string
}

func (p PluginError) Error() string {
	return fmt.Sprintf("plugin error: %s", p.Message)
}

// Retryable wraps a function that only returns an error
// and makes it retryable in the plugin system.
func Retryable[Arg any](function ContextFunc[Arg]) ContextFunc[Arg] {
	return func(ctx context.Context, arg Arg) error {
		err := function(ctx, arg)
		if err != nil {
			return PluginError{
				ErrorCode: providerserverv1.ErrorCode_ERROR_CODE_TRANSIENT,
				Message:   err.Error(),
			}
		}
		return nil
	}
}

// RetryableReturnValue wraps a function that returns a value and an error
// and makes it retryable in the plugin system.
func RetryableReturnValue[Arg any, Value any](
	function ContextFuncReturnValue[Arg, Value],
) ContextFuncReturnValue[Arg, Value] {
	return func(ctx context.Context, arg Arg) (Value, error) {
		val, err := function(ctx, arg)
		if err != nil {
			return val, PluginError{
				ErrorCode: providerserverv1.ErrorCode_ERROR_CODE_TRANSIENT,
				Message:   err.Error(),
			}
		}
		return val, nil
	}
}

// Timeout wraps a function that only returns an error
// and apples a timeout to it.
// Timeouts can be due to transient or permanent issues,
// to retry a timeout error, wrap the timeout function with Retryable.
func Timeout[Arg any](function ContextFunc[Arg], timeout time.Duration) ContextFunc[Arg] {
	return func(ctx context.Context, arg Arg) error {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return function(timeoutCtx, arg)
	}
}

// TimeoutReturnValue wraps a function that returns a value and an error
// and applies a timeout to it.
// Timeouts can be due to transient or permanent issues,
// to retry a timeout error, wrap the timeout function with RetryableReturnValue.
func TimeoutReturnValue[Arg any, Value any](
	function ContextFuncReturnValue[Arg, Value],
	timeout time.Duration,
) ContextFuncReturnValue[Arg, Value] {
	return func(ctx context.Context, arg Arg) (Value, error) {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return function(timeoutCtx, arg)
	}
}
