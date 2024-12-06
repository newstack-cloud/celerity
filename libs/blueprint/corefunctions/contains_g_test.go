package corefunctions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
)

type Contains_G_FunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
	suite.Suite
}

func (s *Contains_G_FunctionTestSuite) SetupTest() {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &core.ParamsImpl{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
}

func (s *Contains_G_FunctionTestSuite) Test_returns_function_runtime_info_with_partial_args() {
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

	s.Require().NoError(err)
	s.Assert().Equal(provider.FunctionRuntimeInfo{
		FunctionName: "contains",
		PartialArgs:  []any{"search for me"},
		ArgsOffset:   1,
	}, output.FunctionInfo)
}

func TestContains_G_FunctionTestSuite(t *testing.T) {
	suite.Run(t, new(Contains_G_FunctionTestSuite))
}
