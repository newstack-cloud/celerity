// Package awsrunner hosts and invokes the Celerity maintenance runner on AWS.
//
// The runner is a Lambda function in the database's private subnets. Both
// halves of the AWS-specific story live here, and both register themselves on
// import: a Host, which delivers Lambda invocations to the cloud-neutral
// executor, and an Invoker, which the CLI calls it through. Importing this
// package is the only thing that ties either binary to AWS.
//
// lambda:Invoke is what makes this work. It is a control-plane call, so it
// reaches a function in a private subnet from an operator's machine with no
// network path to the VPC at all, and the same credentials that just ran the
// deploy are enough.
package awsrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
)

func init() {
	maintenancerunner.RegisterInvoker("aws", func() maintenancerunner.Invoker { return &Invoker{} })
	maintenancerunner.RegisterInvoker("aws-serverless", func() maintenancerunner.Invoker { return &Invoker{} })
}

// lambdaFunctionType is the concrete resource type the runner is deployed as.
const lambdaFunctionType = "aws/lambda/function"

// Invoker calls the runner Lambda.
type Invoker struct{}

// Deployed reports whether the instance has a maintenance runner.
func (i *Invoker) Deployed(instance *state.InstanceState) bool {
	_, err := runnerFunctionName(instance)
	return err == nil
}

// Invoke calls the runner and decodes its response.
func (i *Invoker) Invoke(
	ctx context.Context,
	instance *state.InstanceState,
	req maintenancerunner.Request,
) (*maintenancerunner.Response, error) {
	functionName, err := runnerFunctionName(instance)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encoding the maintenance request: %w", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, runnerRegionOption(instance)...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	out, err := lambda.NewFromConfig(awsCfg).Invoke(ctx, &lambda.InvokeInput{
		FunctionName: &functionName,
		// Synchronous: the CLI has nothing useful to do until the operation has
		// either happened or failed, and an operator watching a command wants
		// the outcome rather than an acknowledgement.
		InvocationType: lambdatypes.InvocationTypeRequestResponse,
		Payload:        payload,
	})
	if err != nil {
		return nil, fmt.Errorf(
			"invoking the maintenance runner %s: %w "+
				"(the deploying principal needs lambda:InvokeFunction on it)",
			functionName, err)
	}

	// A function error is the runner crashing rather than an operation failing,
	// which the executor reports in the response body. The payload here is
	// Lambda's error envelope, which is the most useful thing to show.
	if out.FunctionError != nil {
		return nil, fmt.Errorf(
			"the maintenance runner failed to run: %s: %s",
			*out.FunctionError, string(out.Payload))
	}

	var resp maintenancerunner.Response
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return nil, fmt.Errorf("decoding the maintenance runner's response: %w", err)
	}
	return &resp, nil
}

// The region the runner is deployed in, taken from its own ARN in instance state.
//
// A Lambda can only be invoked in the region that holds it, so the deployed resource is
// the authoritative source and needs no configuration to agree with. Without this the
// default chain has only the environment to go on, and an operator whose deploy config
// names a region but whose shell does not gets "Missing Region" from a command that had
// the answer in front of it. An empty result leaves the default chain untouched.
func runnerRegionOption(instance *state.InstanceState) []func(*awsconfig.LoadOptions) error {
	region := runnerRegion(instance)
	if region == "" {
		return nil
	}

	return []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
}

func runnerRegion(instance *state.InstanceState) string {
	if instance == nil {
		return ""
	}

	for _, resource := range instance.Resources {
		if resource == nil || resource.Type != lambdaFunctionType {
			continue
		}
		if annotation(resource, maintenancerunner.RunnerAnnotation) !=
			maintenancerunner.RunnerAnnotationValue {
			continue
		}

		return regionFromARN(specString(resource, "arn"))
	}

	return ""
}

// Pulls the region out of an ARN of the form arn:aws:lambda:<region>:<account>:function:<name>.
func regionFromARN(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 || parts[0] != "arn" {
		return ""
	}

	return parts[3]
}

// Finds the deployed runner in instance state by the annotation the transformer
// stamps on it, rather than by a name convention the CLI would have to keep in
// step with the transformer.
func runnerFunctionName(instance *state.InstanceState) (string, error) {
	if instance == nil {
		return "", fmt.Errorf("no instance state available")
	}
	for _, resource := range instance.Resources {
		if resource == nil || resource.Type != lambdaFunctionType {
			continue
		}
		if annotation(resource, maintenancerunner.RunnerAnnotation) !=
			maintenancerunner.RunnerAnnotationValue {
			continue
		}
		if name := specString(resource, "functionName"); name != "" {
			return name, nil
		}
		if arn := specString(resource, "arn"); arn != "" {
			return arn, nil
		}
		return "", fmt.Errorf("the deployed maintenance runner has no function name in state")
	}
	return "", fmt.Errorf("this instance has no deployed maintenance runner")
}

func annotation(resource *state.ResourceState, key string) string {
	if resource.Metadata == nil {
		return ""
	}
	node := resource.Metadata.Annotations[key]
	if node == nil || node.Scalar == nil {
		return ""
	}
	return node.Scalar.ToString()
}

func specString(resource *state.ResourceState, field string) string {
	spec := resource.SpecData
	if spec == nil || spec.Fields == nil {
		return ""
	}
	node := spec.Fields[field]
	if node == nil || node.Scalar == nil {
		return ""
	}
	return node.Scalar.ToString()
}
