package corefunctions

import (
	"context"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type Contains_G_FunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
}

var _ = Suite(&Contains_G_FunctionTestSuite{})

func (s *Contains_G_FunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *Contains_G_FunctionTestSuite) Test_returns_function_runtime_info_with_partial_args(c *C) {
	contains_G_Func := NewContains_G_Function()
	s.callStack.Push(&function.Call{
		FunctionName: "contains_g",
	})
	output, err := contains_G_Func.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				"search for me",
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	c.Assert(output.FunctionInfo, DeepEquals, provider.FunctionRuntimeInfo{
		FunctionName: "contains",
		PartialArgs:  []any{"search for me"},
		ArgsOffset:   1,
	})
}
