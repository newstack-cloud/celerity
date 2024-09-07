package corefunctions

import (
	"context"
	"fmt"

	"github.com/two-hundred/celerity/libs/blueprint/function"
	"github.com/two-hundred/celerity/libs/blueprint/internal"
	"github.com/two-hundred/celerity/libs/blueprint/provider"
	. "gopkg.in/check.v1"
)

type CWDFunctionTestSuite struct {
	callStack         function.Stack
	callContext       *functionCallContextMock
	getWorkingDir     WorkingDirResolver
	getWorkingDirFail WorkingDirResolver
}

var _ = Suite(&CWDFunctionTestSuite{})

func (s *CWDFunctionTestSuite) SetUpTest(c *C) {
	s.callStack = function.NewStack()
	s.callContext = &functionCallContextMock{
		params: &blueprintParamsMock{},
		registry: &internal.FunctionRegistryMock{
			Functions: map[string]provider.Function{},
			CallStack: s.callStack,
		},
		callStack: s.callStack,
	}
	s.getWorkingDir = func() (string, error) {
		return "/home/user", nil
	}
	s.getWorkingDirFail = func() (string, error) {
		return "", fmt.Errorf("failed to resolve working directory")
	}
}

func (s *CWDFunctionTestSuite) Test_gets_current_working_directory(c *C) {
	notFunc := NewCWDFunction(s.getWorkingDir)
	s.callStack.Push(&function.Call{
		FunctionName: "cwd",
	})
	output, err := notFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args:    []any{},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, IsNil)
	outputStr, isStr := output.ResponseData.(string)
	c.Assert(isStr, Equals, true)
	c.Assert(outputStr, Equals, "/home/user")
}

func (s *CWDFunctionTestSuite) Test_returns_func_error_on_failure_to_get_current_working_directory(c *C) {
	notFunc := NewCWDFunction(s.getWorkingDirFail)
	s.callStack.Push(&function.Call{
		FunctionName: "cwd",
	})
	_, err := notFunc.Call(context.TODO(), &provider.FunctionCallInput{
		Arguments: &functionCallArgsMock{
			args:    []any{},
			callCtx: s.callContext,
		},
		CallContext: s.callContext,
	})

	c.Assert(err, NotNil)
	funcErr, isFuncErr := err.(*function.FuncCallError)
	c.Assert(isFuncErr, Equals, true)
	c.Assert(funcErr.Message, Equals, "unable to get current working directory: failed to resolve working directory")
	c.Assert(funcErr.CallStack, DeepEquals, []*function.Call{
		{
			FunctionName: "cwd",
		},
	})
	c.Assert(funcErr.Code, Equals, function.FuncCallErrorCodeFunctionCall)
}
