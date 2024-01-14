package substitutions

import (
	"fmt"
	"regexp"
)

// The reference patterns in this file are used to efficiently
// check if a full substitution string is an exact match for a reference.
// References in function calls are parsed using a hand-rolled
// sequential character processing state machine.

const (
	namePattern              = `[A-Za-z_][A-Za-z0-9_-]+`
	stringLiteralNamePattern = `([A-Za-z0-9_-]|\.)+`
)

func nameAccessorPattern(index int) string {
	return fmt.Sprintf(`((\.(?P<Name%d>%s))|(\["(?P<NameInLiteral%d>%s)"\]))`, index, namePattern, index, stringLiteralNamePattern)
}

var (
	// NamePattern is the pattern that a name/identifier
	// must match in a substitution.
	NamePattern = regexp.MustCompile(
		`^` + namePattern + `$`,
	)
	// NameStringLiteralPattern is the pattern that a name/identifier
	// must match in a substitution when it is the string literal part
	// of a object key accessor (i.e. key.v1 in "metadata.annotations[\"key.v1\"]").
	NameStringLiteralPattern = regexp.MustCompile(
		`^` + stringLiteralNamePattern + `$`,
	)
	// ResourceReferencePattern is the pattern that a resource
	// reference must match.
	//
	// Some examples that match the resource pattern are:
	// - saveOrderFunction
	//   - This will resolve the field assigned as the ID for the resource.
	// - resources.saveOrderFunction
	// 	 - This will also resolve the field assigned as the ID for the resource
	//     in the resource provider implementation mapping to .state.{idField}.
	// - resources.saveOrderFunction.state.functionArn
	// - resources.save_order_function.state.endpoints[].host
	//   - Shorthand that will resolve the host of the first endpoint in the array.
	// - resources.saveOrderFunction.state.endpoints[0].host
	// - resources.saveOrderFunction.spec.functionName
	// - resources.save-order-function.metadata.custom.apiEndpoint
	// - resources.save-order-function.metadata.displayName
	// - resources.saveOrderFunction.state.configurations[0][1].concurrency
	// - resources.saveOrderFunction.metadata.annotations["annotationKey.v1"]
	// - resources.saveOrderFunction.state["stateValue.v1"].value
	// - resources["save-order-function.v1"].state.functionArn
	//
	// Resources do not have to be referenced with the "resources" prefix,
	// but using the prefix is recommended to avoid ambiguity.
	// All other referenced types must be referenced with the prefix.
	ResourceReferencePattern = regexp.MustCompile(
		`^((resources` + nameAccessorPattern(0) + `)|(?P<NameWithoutNamespace>` + namePattern + `))` +
			`(?P<Path>\.(metadata\.displayName|((state|spec|metadata\.(labels|custom|annotations))` + nameAccessorPattern(1) +
			`(` + nameAccessorPattern(2) + `|\[\d*\])*)))?$`,
	)

	// VariableReferencePattern is the pattern that a variable
	// reference must match.
	//
	// Some examples that match the variable pattern are:
	// - variables.environment
	// - variables.enableFeatureV2
	// - variables.enable_feature_v3
	// - variables.function-name
	// - variables["common.app.v1.name"]
	//
	// Variables must be referenced with the "variables" prefix.
	VariableReferencePattern = regexp.MustCompile(
		`^variables` + nameAccessorPattern(0) + `$`,
	)

	// DataSourceReferencePattern is the pattern that a data source
	// reference must match.
	//
	// Some examples that match the data source pattern are:
	// - datasources.network.vpc
	// - datasources.network.endpoints[]
	//   - Shorthand that will resolve the host of the first endpoint in the array.
	// - datasources.network.endpoints[0]
	// - datasources.core-infra.queueUrl
	// - datasources.coreInfra1.topics[1]
	// - datasources["core-infra.v1"].queueUrl
	// - datasources["coreInfra.v1"]["topic.v2"][0]
	//
	// Data sources must be referenced with the "datasources" prefix.
	// Data source export fields can be primitives or arrays of primitives
	// only, see the specification.
	DataSourceReferencePattern = regexp.MustCompile(
		`^datasources` + nameAccessorPattern(0) +
			`(?P<Path>` + nameAccessorPattern(1) + `(\[\d*\])?)$`,
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
	// - children["core-infrastructure.v1"].cacheNodes[].host
	// - children["coreInfrastructure.v1"]["topic.v2"].arn
	//
	// Child blueprints must be referenced with the "children" prefix.
	ChildReferencePattern = regexp.MustCompile(
		`^children` + nameAccessorPattern(0) +
			`(?P<Path>(` + nameAccessorPattern(1) + `|\[\d*\])+)$`,
	)
)
