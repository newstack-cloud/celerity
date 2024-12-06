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

type AndFunctionTestSuite struct {
	callStack   function.Stack
	callContext *functionCallContextMock
	suite.Suite
}

func (s *AndFunctionTestSuite) SetupTest() {
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

func (s *AndFunctionTestSuite) Test_applies_logical_and() {
	andFunc := NewAndFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "and",
	})
	output, err := andFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				false,
				true,
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	s.Require().NoError(err)
	outputBool, isBool := output.ResponseData.(bool)
	s.Assert().True(isBool)
	s.Assert().False(outputBool)
}

func (s *AndFunctionTestSuite) Test_returns_func_error_for_invalid_input() {
	andFunc := NewAndFunction()
	s.callStack.Push(&function.Call{
		FunctionName: "and",
	})
	_, err := andFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args: []any{
				true,
				// A boolean is expected here, not an integer.
				985043,
			},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	s.Require().Error(err)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	s.Assert().True(isFuncErr)
	s.Assert().Equal("argument at index 1 is of type int, but target is of type bool", funcErr.Message)
	s.Assert().Equal([]*function.Call{{FunctionName: "and"}}, funcErr.CallStack)
	s.Assert().Equal(function.FuncCallErrorCodeInvalidArgumentType, funcErr.Code)
}

func TestAndTestSuite(t *testing.T) {
	suite.Run(t, new(AndFunctionTestSuite))
}
