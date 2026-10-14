// Package deploydb resolves a database connection from a deployed blueprint
// instance's state so the CLI can run schema migrations against a deployed
// environment.
//
// Resolution is provider-specific: how the concrete database and credential
// resources appear in state, and where credentials are fetched from, differ per
// cloud. Celerity is not AWS-only, so resolution is modular. A Resolver is
// selected by the deploy target (see ResolverFor), with one implementation per
// provider (aws.go today, with gcloud and azure to follow).
package deploydb

import (
	"context"
	"fmt"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
)

// ResolvedConnection is a ready-to-use database connection derived from
// deployed state, with credentials already resolved from the provider's secret
// store.
type ResolvedConnection struct {
	// ConnectionString is the full connection URL (contains credentials).
	ConnectionString string
	// Target is a human-readable host/db summary, safe to print (no credentials).
	Target string
}

// ResolveInput names the database to resolve a connection for.
type ResolveInput struct {
	// SQLDatabaseName is the celerity/sqlDatabase resource's name in the
	// blueprint.
	SQLDatabaseName string

	// Engine is the database engine ("postgres", "mysql"), which determines the
	// shape of the connection string as well as the driver.
	Engine string

	// FallbackDatabase is the database name to use when deployed state does not
	// carry one.
	FallbackDatabase string
}

// Resolver derives a database connection from deployed instance state for a
// particular deploy target's provider.
type Resolver interface {
	Resolve(
		ctx context.Context,
		instance *state.InstanceState,
		input ResolveInput,
	) (*ResolvedConnection, error)
}

// ResolverFor returns the connection resolver for a deploy target, or an error
// if that target's resolution is not supported yet.
func ResolverFor(deployTarget string) (Resolver, error) {
	switch deployTarget {
	case "aws", "aws-serverless":
		return &awsResolver{}, nil
	case "gcloud", "gcloud-serverless":
		return nil, fmt.Errorf("resolving a deployed database connection for %q is not yet supported (GCP planned for v1); use --url", deployTarget)
	case "azure", "azure-serverless":
		return nil, fmt.Errorf("resolving a deployed database connection for %q is not yet supported (Azure planned for v1); use --url", deployTarget)
	default:
		return nil, fmt.Errorf("cannot resolve a deployed database connection for deploy target %q; use --url", deployTarget)
	}
}

// --- shared state-navigation helpers (provider-agnostic) ---

func abstractName(resource *state.ResourceState) string {
	if resource == nil || resource.Metadata == nil {
		return ""
	}
	return nodeString(resource.Metadata.Annotations[abstractNameAnnotation])
}

// nodeString walks a dotted field path from a mapping node and returns the
// scalar string at the end, or "" if any segment is missing.
func nodeString(node *core.MappingNode, path ...string) string {
	for _, field := range path {
		if node == nil || node.Fields == nil {
			return ""
		}
		node = node.Fields[field]
	}
	if node == nil || node.Scalar == nil {
		return ""
	}
	return node.Scalar.ToString()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Stamped by the Celerity transformer on every concrete resource, holding the
// logical name of the abstract resource it was generated from.
const abstractNameAnnotation = "bluelink.transform.source.abstractName"

// secretRoleAnnotation records which credential a secret generated for a
// database holds, and secretRoleAppUser marks the application user's.
//
// A celerity/sqlDatabase produces more than one secret, all carrying the same
// abstract name, and migrations must run as the owner. The application user has
// no rights to create or alter anything, and is not even guaranteed to exist
// yet. Both constants mirror the transformer's
// resources/sqldatabase.AnnotationKeySecretRole.
const (
	secretRoleAnnotation = "celerity.sqlDatabase.secret.role"
	secretRoleAppUser    = "app-user"
)

// Returns the role a secret resource holds, or "" when it is unmarked. Every
// secret from a deploy predating the application user is unmarked, and those are
// all master credentials.
func secretRole(resource *state.ResourceState) string {
	if resource == nil || resource.Metadata == nil {
		return ""
	}
	return nodeString(resource.Metadata.Annotations[secretRoleAnnotation])
}
