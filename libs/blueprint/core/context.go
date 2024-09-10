package core

import (
	"context"
	"errors"
)

// ContextKey provides a unique key type for blueprint execution
// context values.
type ContextKey string

func (c ContextKey) String() string {
	return "blueprint context key " + string(c)
}

const (
	// BlueprintInstanceIDKey is the key used to store the blueprint instance ID
	// in the context for a blueprint execution.
	BlueprintInstanceIDKey = ContextKey("blueprintInstanceID")
)

// BlueprintInstanceIDFromContext retrieves the current blueprint instance ID from the context
// passed through when loading and carrying out an action for a blueprint.
func BlueprintInstanceIDFromContext(ctx context.Context) (string, error) {
	instanceID, ok := ctx.Value(BlueprintInstanceIDKey).(string)
	if !ok {
		return "", errors.New("no blueprint instance ID found in context")
	}

	return instanceID, nil
}
