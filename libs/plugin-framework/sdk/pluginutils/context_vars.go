package pluginutils

import "github.com/newstack-cloud/celerity/libs/blueprint/core"

// SessionIDKey is the plain text key used to store the session ID in a Go context
// or as a part of the blueprint framework's context variables.
const SessionIDKey = "celerity.sessionId"

// ContextKey provides a unique key type for Celerity context variables.
type ContextKey string

func (c ContextKey) String() string {
	return "celerity context key " + string(c)
}

var (
	// ContextSessionIDKey is the context key used to store the session ID
	// in a Go context.
	ContextSessionIDKey = core.ContextKey(SessionIDKey)
)
