package container

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type LoaderTestSuite struct {
	suite.Suite
}

func (s *LoaderTestSuite) Test_loads_container_from_input_spec_file_without_any_issues() {
}

func (s *LoaderTestSuite) Test_loads_container_from_input_spec_string_without_any_issues() {}

func (s *LoaderTestSuite) Test_validates_spec_from_input_spec_file_without_any_issues() {
}

func (s *LoaderTestSuite) Test_validates_spec_from_input_spec_string_without_any_issues() {}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_is_invalid() {
	// This is for when the spec is invalid JSON/YAML, as test coverage for specific formats
	// is handled by the schema package, we just need to ensure that the error is reported
	// for either format.
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_fails_schema_specific_validation() {
	// This is for when the spec is valid JSON/YAML, but fails validation against the schema.
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_the_provided_spec_contains_unsupported_variable_types() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_when_there_is_a_mismatch_between_variable_type_and_value() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_when_a_given_custom_variable_value_is_invalid() {
}

func (s *LoaderTestSuite) Test_reports_expected_error_when_a_given_custom_variable_type_provider_is_missing() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_when_a_given_resource_provider_is_missing() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_for_a_missing_resource() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_for_a_resource_with_an_invalid_spec() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_when_a_given_data_source_provider_is_missing() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_for_a_missing_data_source() {

}

func (s *LoaderTestSuite) Test_reports_expected_error_for_unsupported_exported_fields_in_a_data_source() {

}

func TestLoaderTestSuite(t *testing.T) {
	suite.Run(t, new(LoaderTestSuite))
}
