package awsrunner

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scalar(s string) *core.MappingNode {
	return &core.MappingNode{Scalar: core.ScalarFromString(s)}
}

func lambdaResource(annotations map[string]string, spec map[string]*core.MappingNode) *state.ResourceState {
	nodes := map[string]*core.MappingNode{}
	for key, value := range annotations {
		nodes[key] = scalar(value)
	}
	return &state.ResourceState{
		Type:     lambdaFunctionType,
		SpecData: &core.MappingNode{Fields: spec},
		Metadata: &state.ResourceMetadataState{Annotations: nodes},
	}
}

func instance(resources ...*state.ResourceState) *state.InstanceState {
	m := map[string]*state.ResourceState{}
	for i, r := range resources {
		m[string(rune('a'+i))] = r
	}
	return &state.InstanceState{Resources: m}
}

func runnerResource() *state.ResourceState {
	return lambdaResource(
		map[string]string{
			maintenancerunner.RunnerAnnotation: maintenancerunner.RunnerAnnotationValue,
		},
		map[string]*core.MappingNode{"functionName": scalar("orders-maintenance-runner")},
	)
}

// An application's functions are all aws/lambda/function, so the runner is
// found by the annotation the transformer stamps on it rather than by type or
// by a name convention the CLI would have to keep in step with.
func TestFindsTheRunnerAmongTheApplicationsFunctions(t *testing.T) {
	inst := instance(
		lambdaResource(nil, map[string]*core.MappingNode{"functionName": scalar("saveOrder")}),
		runnerResource(),
		lambdaResource(nil, map[string]*core.MappingNode{"functionName": scalar("health")}),
	)

	name, err := runnerFunctionName(inst)

	require.NoError(t, err)
	assert.Equal(t, "orders-maintenance-runner", name)
}

// An instance deployed before runners existed, or one whose database is
// reachable without one, has none, and that is not an error on the auto path.
func TestAnInstanceWithNoRunnerIsReported(t *testing.T) {
	inst := instance(lambdaResource(nil, map[string]*core.MappingNode{"functionName": scalar("saveOrder")}))

	_, err := runnerFunctionName(inst)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deployed maintenance runner")
	assert.False(t, (&Invoker{}).Deployed(inst))
	assert.True(t, (&Invoker{}).Deployed(instance(runnerResource())))
}

// Lambda accepts an ARN wherever it accepts a function name, so state that
// carries only the ARN is still usable.
func TestFallsBackToTheARN(t *testing.T) {
	inst := instance(lambdaResource(
		map[string]string{
			maintenancerunner.RunnerAnnotation: maintenancerunner.RunnerAnnotationValue,
		},
		map[string]*core.MappingNode{
			"arn": scalar("arn:aws:lambda:eu-west-2:1:function:orders-maintenance-runner"),
		},
	))

	name, err := runnerFunctionName(inst)

	require.NoError(t, err)
	assert.Equal(t, "arn:aws:lambda:eu-west-2:1:function:orders-maintenance-runner", name)
}

// Detection keys on a variable the execution environment sets for every
// runtime, rather than a function name or region a user could set anywhere.
func TestDetectsALambdaExecutionEnvironment(t *testing.T) {
	h := &host{}
	assert.Equal(t, "aws", h.Name())
	assert.False(t, h.Detect(), "not running on Lambda")

	t.Setenv("AWS_LAMBDA_RUNTIME_API", "127.0.0.1:9001")

	assert.True(t, h.Detect())
}

// Both registries are populated from init, and a runner or CLI that silently
// registered nothing would fail much later with a confusing message.
func TestRegistersItselfAsAHostAndAnInvoker(t *testing.T) {
	for _, target := range []string{"aws", "aws-serverless"} {
		invoker, err := maintenancerunner.InvokerFor(target)
		require.NoError(t, err, target)
		assert.NotNil(t, invoker)
	}

	assert.Contains(t, maintenancerunner.HostNames(), "aws",
		"importing this package must be all it takes for the runner to serve on Lambda")
}
