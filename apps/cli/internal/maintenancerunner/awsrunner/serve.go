package awsrunner

import (
	"context"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn"
	"github.com/newstack-cloud/celerity/apps/cli/internal/dbconn/awscreds"
	"github.com/newstack-cloud/celerity/apps/cli/internal/maintenancerunner"
	"go.uber.org/zap"
)

func init() {
	maintenancerunner.RegisterHost(&host{})
}

// host runs the maintenance runner on AWS Lambda.
type host struct{}

func (h *host) Name() string { return "aws" }

// Detect reports whether the process is a Lambda invocation.
//
// AWS_LAMBDA_RUNTIME_API is set by the execution environment itself, for every
// runtime, and is what the Lambda SDK talks to, so it is present exactly when
// this host can serve. A function name or region, by contrast, a user could set
// anywhere.
func (h *host) Detect() bool {
	return os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
}

func (h *host) Credentials() dbconn.CredentialSource { return awscreds.New() }

// Serve hands Lambda invocations to the runner. It does not return.
func (h *host) Serve(handle maintenancerunner.Handler, _ *zap.Logger) error {
	lambda.Start(func(
		ctx context.Context,
		req maintenancerunner.Request,
	) (*maintenancerunner.Response, error) {
		// Always a response, never an error. An operation that fails is a
		// successful invocation carrying a report the CLI can print, where an
		// error here would reach the operator as a Lambda stack trace instead.
		return handle(ctx, req), nil
	})
	return nil
}
