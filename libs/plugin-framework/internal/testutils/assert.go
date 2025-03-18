package testutils

import (
	"fmt"

	"github.com/stretchr/testify/suite"
	"github.com/two-hundred/celerity/libs/blueprint/core"
	"github.com/two-hundred/celerity/libs/plugin-framework/errorsv1"
	"github.com/two-hundred/celerity/libs/plugin-framework/sharedtypesv1"
)

// AssertConfigDefinitionEquals asserts that two core config definitions
// are equal.
// This treats nil and empty slices in the config field definitions
// as equal.
func AssertConfigDefinitionEquals(
	expected *core.ConfigDefinition,
	actual *core.ConfigDefinition,
	testSuite *suite.Suite,
) {
	for key, expectedField := range expected.Fields {
		actualField, ok := actual.Fields[key]
		testSuite.Assert().True(ok)
		testSuite.Assert().Equal(expectedField.Type, actualField.Type)
		testSuite.Assert().Equal(expectedField.Label, actualField.Label)
		testSuite.Assert().Equal(expectedField.Description, actualField.Description)
		testSuite.Assert().Equal(expectedField.Required, actualField.Required)
		testSuite.Assert().Equal(expectedField.DefaultValue, actualField.DefaultValue)
		AssertSlicesEqual(expectedField.Examples, actualField.Examples, testSuite)
		AssertSlicesEqual(expectedField.AllowedValues, actualField.AllowedValues, testSuite)
	}
}

// AssertInvalidHost asserts that the given error is an invalid host error
// from a plugin method call response.
func AssertInvalidHost(
	respErr error,
	action errorsv1.PluginAction,
	invalidHostID string,
	testSuite *suite.Suite,
) {
	testSuite.Require().Error(respErr)
	pluginRespErr, isPluginRespErr := respErr.(*errorsv1.PluginResponseError)
	testSuite.Assert().True(isPluginRespErr)
	testSuite.Assert().Equal(
		action,
		pluginRespErr.Action,
	)
	testSuite.Assert().Equal(
		sharedtypesv1.ErrorCode_ERROR_CODE_UNEXPECTED,
		pluginRespErr.Code,
	)
	testSuite.Assert().Equal(
		fmt.Sprintf("invalid host ID %q", invalidHostID),
		pluginRespErr.Message,
	)
}

// AssertSlicesEqual asserts that two slices are equal.
// Nil and empty slices are considered equal.
// The order of the elements in the slices must be the same.
func AssertSlicesEqual[Item any](
	expected []Item,
	actual []Item,
	testSuite *suite.Suite,
) {
	if expected != nil {
		expectedCopy := make([]Item, len(expected))
		copy(expectedCopy, expected)

		actualCopy := make([]Item, len(actual))
		copy(actualCopy, actual)

		testSuite.Assert().Equal(expectedCopy, actualCopy)
	} else {
		testSuite.Assert().Empty(actual)
	}
}
