package container

import (
	"regexp"
	"strings"

	"github.com/two-hundred/celerity/libs/common/pkg/core"
)

// ValidateReference validates a reference in a blueprint,
// a reference can be a reference to a variable, resource, child blueprint or data source.
// This validation does not validate that the reference can be resolved,
// as this validation will normally be carried out at an early stage before information
// is available about what resources, variables, data sources or child blueprints are available.
func ValidateReference(reference string, context string, hasAccessTo []Referenceable) error {
	if strings.HasPrefix(reference, "variables.") {
		return validateVariableReference(reference, context, hasAccessTo)
	}

	if strings.HasPrefix(reference, "dataSources.") {
		return validateDataSourceReference(reference, context, hasAccessTo)
	}

	if strings.HasPrefix(reference, "children.") {
		return validateChildBlueprintReference(reference, context, hasAccessTo)
	}

	// Resource references are used for all other cases as they can be made
	// with or without the "resources." prefix.
	return validateResourceReference(reference, context, hasAccessTo)
}

func validateVariableReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableVariable) {
		return errReferenceContextAccess(reference, context, ReferenceableVariable)
	}

	if !VariableReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableVariable)
	}

	return nil
}

func validateDataSourceReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableDataSource) {
		return errReferenceContextAccess(reference, context, ReferenceableDataSource)
	}

	if !DataSourceReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableDataSource)
	}

	return nil
}

func validateChildBlueprintReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableChild) {
		return errReferenceContextAccess(reference, context, ReferenceableChild)
	}

	if !ChildReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableChild)
	}

	return nil
}

func validateResourceReference(reference string, context string, hasAccessTo []Referenceable) error {
	if !core.SliceContainsComparable(hasAccessTo, ReferenceableResource) {
		return errReferenceContextAccess(reference, context, ReferenceableResource)
	}

	if !ResourceReferencePattern.MatchString(reference) {
		return errInvalidReferencePattern(reference, context, ReferenceableResource)
	}

	return nil
}

// Referencable is a type that can be referenced in a blueprint.
type Referenceable string

const (
	// ReferenceableResource signifies that a resource
	// can be referenced for a given context in a blueprint.
	ReferenceableResource Referenceable = "resource"
	// ReferenceableVariable signifies that a variable
	// can be referenced for a given context in a blueprint.
	ReferenceableVariable Referenceable = "variable"
	// ReferenceableDataSource signifies that a data source
	// can be referenced for a given context in a blueprint.
	ReferenceableDataSource Referenceable = "dataSource"
	// ReferenceableChild signifies that a child blueprint
	// can be referenced for a given context in a blueprint.
	ReferenceableChild Referenceable = "child"
)

func referenceableLabel(referenceable Referenceable) string {
	switch referenceable {
	case ReferenceableResource:
		return "resource"
	case ReferenceableVariable:
		return "variable"
	case ReferenceableDataSource:
		return "data source"
	case ReferenceableChild:
		return "child blueprint"
	default:
		return "unknown"
	}
}

var (
	// ResourceReferencePattern is the pattern that a resource
	// reference must match.
	//
	// Some examples that match the resource pattern are:
	// - saveOrderFunction
	//   - This will resolve the field assigned as the ID for the resource.
	// - resources.saveOrderFunction
	// 	 - This will also resolve the field assigned as the ID for the resource.
	// - resources.saveOrderFunction.state.functionArn
	// - resources.save_order_function.state.endpoints[].host
	//   - Shorthand that will resolve the host of the first endpoint in the array.
	// - resources.saveOrderFunction.state.endpoints[0].host
	// - resources.saveOrderFunction.spec.functionName
	// - resources.save-order-function.metadata.custom.apiEndpoint
	// - resources.save-order-function.metadata.displayName
	// - resources.saveOrderFunction.state.confgiruations[0][1].concurrency
	//
	// Resources do not have to be referenced with the "resources" prefix,
	// but using the prefix is recommended to avoid ambiguity.
	// All other referenced types must be referenced with the prefix.
	ResourceReferencePattern = regexp.MustCompile(
		`^(resources\.)?[A-Za-z_][A-Za-z0-9_-]+(\.(metadata\.displayName|` +
			`(state|spec|metadata\.(labels|custom|annotations))\.([A-Za-z0-9\.]|\[\d*\])*))?$`,
	)

	// VariableReferencePattern is the pattern that a variable
	// reference must match.
	//
	// Some examples that match the variable pattern are:
	// - variables.environment
	// - variables.enableFeatureV2
	// - variables.enable_feature_v3
	// - variables.function-name
	//
	// Variables must be referenced with the "variables" prefix.
	VariableReferencePattern = regexp.MustCompile(
		`^variables\.[A-Za-z_][A-Za-z0-9_-]+$`,
	)

	// DataSourceReferencePattern is the pattern that a data source
	// reference must match.
	//
	// Some examples that match the data source pattern are:
	// - dataSources.network.vpc
	// - dataSources.network.endpoints[]
	//   - Shorthand that will resolve the host of the first endpoint in the array.
	// - dataSources.network.endpoints[0]
	// - dataSources.core-infra.queueUrl
	// - dataSources.coreInfra1.topics[1]
	//
	// Data sources must be referenced with the "dataSources" prefix.
	// Data source export fields can be primitives or arrays of primitives
	// only, see the specification.
	DataSourceReferencePattern = regexp.MustCompile(
		`^dataSources\.[A-Za-z_][A-Za-z0-9_-]\.[A-Za-z_][A-Za-z0-9_-]$`,
	)

	// ChildReferencePattern is the pattern that a child blueprint
	// reference must match.
	//
	// Some examples that match the child blueprint pattern are:
	// - children.coreInfrastructure.ordersTopicId
	// - children.coreInfrastructure.cacheNodes[].host
	// - children.core-infrastructure.cacheNodes[0].host
	// - children.topics.orderTopicInfo.type
	// - children.topics.order_topic_info.arn
	// - children.topics.configurations[1]
	// - children.topics.configurations[1][2].messagesPerSecond
	//
	// Child blueprints must be referenced with the "children" prefix.
	ChildReferencePattern = regexp.MustCompile(
		`^children\.[A-Za-z_][A-Za-z0-9_-]+\.[A-Za-z_][A-Za-z0-9_-]+(\[\d*\])?(([A-Za-z0-9\.]|\[\d*\])*)?$`,
	)
)
