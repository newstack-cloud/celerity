package plugintestutils

import (
	"runtime"
	"strings"
	"sync"

	"github.com/stretchr/testify/suite"
)

// MockCalls is a lightweight test helper for tracking
// calls to methods in a mock implementation of an upstream provider
// service interface.
// This can be embedded in a service-specific mock struct
// to track calls to methods and make assertions on them in tests.
// This struct is safe for concurrent use when multiple calls are made
// across goroutines at the same time.
//
// Example:
//
//	type MyServiceMock struct {
//	    MockCalls
//	    // Other fields specific to the mock service
//	}
//
//	// Other methods specific to the mock service
//
//	func (m *MyServiceMock) MyMethod(arg1 string, arg2 int) {
//	    m.RegisterCall(arg1, arg2)
//	    // Implementation of the method
//	}
type MockCalls struct {
	calls map[string][][]any
	mu    sync.RWMutex
}

// A placeholder type for any value
// that can be used in assertions for method calls.
type anyType struct{}

// Any is a placeholder value for any value
// that can be used in assertions for method calls
// for a value that can be ignored.
var Any = anyType{}

// RegisterCall registers a call for the current method with the given arguments.
func (m *MockCalls) RegisterCall(args ...any) {
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		fullMethodName := details.Name()
		parts := strings.Split(fullMethodName, ".")
		// Get the last part as the method name
		methodName := parts[len(parts)-1]
		m.RegisterNamedCall(methodName, args...)
	}
}

// RegisterNamedCall registers a call to a method with the given name.
func (m *MockCalls) RegisterNamedCall(methodName string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls == nil {
		m.calls = map[string][][]any{}
	}

	m.calls[methodName] = append(m.calls[methodName], args)
}

// AssertCalled asserts that a method with the given name was called at least once.
func (m *MockCalls) AssertCalled(s *suite.Suite, methodName string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.calls == nil {
		s.Assert().Fail(
			"Expected method %s to be called, but no calls were registered",
			methodName,
		)
		return
	}

	calls, hasCalls := m.calls[methodName]
	s.Assert().True(hasCalls && len(calls) > 0, "Expected method %s to be called", methodName)
}

// AssertCalledWith asserts that a method with the given name was called
// with the specific arguments for the provided call index (callNumber - 1).
func (m *MockCalls) AssertCalledWith(s *suite.Suite, methodName string, callIndex int, args ...any) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.calls == nil {
		s.Assert().Fail(
			"Expected method %s to be called, but no calls were registered",
			methodName,
		)
		return
	}

	calls, hasCalls := m.calls[methodName]
	s.Assert().True(hasCalls && len(calls) > 0, "Expected method %s to be called", methodName)

	if callIndex >= len(calls) {
		s.Assert().Fail(
			"Expected method %s to be called at least %d times to assert call at index %d",
			methodName,
			callIndex+1,
			callIndex,
		)
	}

	callArgs := calls[callIndex]
	for i, arg := range callArgs {
		expected := args[i]
		if expected != Any {
			s.Assert().Equal(
				expected,
				arg,
				"Method arg at index %d does not match expected value",
				i,
			)
		}
	}
}

// AssertNotCalled asserts that a method with the given name was not called.
// This is useful to ensure that a method was not invoked during a test.
func (m *MockCalls) AssertNotCalled(s *suite.Suite, methodName string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.calls == nil {
		// No assertion needed if no calls were registered.
		return
	}

	s.Assert().False(
		len(m.calls[methodName]) > 0,
		"Expected method %s to not be called",
		methodName,
	)
}
