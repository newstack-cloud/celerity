// Package engine gives the CLI its deploy engine client.
//
// The client, its interface and the error simplification all live in
// deploy-cli-sdk now, shared with the other CLIs built on it. This package is
// the seam that remains: it re-exports them under the import path the CLI's
// commands already use, so the move needed no change at any call site.
//
// Creating the client is the part worth not duplicating. It resolves the
// endpoint and connect protocol from configuration, loads the engine auth
// config file and handles all three auth methods -- behaviour every command
// depends on and which is easy to get subtly wrong in a second copy.
package engine

import (
	sdkengine "github.com/newstack-cloud/deploy-cli-sdk/engine"
)

// DeployEngine is the deploy engine's v1 API, as implemented by the client.
type DeployEngine = sdkengine.DeployEngine

// Create builds a deploy engine client from the CLI's configuration.
var Create = sdkengine.Create

// SimplifyError turns a transport or API error into something an operator can
// act on.
var SimplifyError = sdkengine.SimplifyError
