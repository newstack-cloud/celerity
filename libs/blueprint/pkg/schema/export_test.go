package schema

import (
	"encoding/json"
	"fmt"
	"os"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"
)

type ExportTestSuite struct {
	specFixtures map[string][]byte
}

var _ = Suite(&ExportTestSuite{})

func (s *ExportTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string][]byte)
	fixturesToLoad := map[string]string{
		"passYAML":              "__testdata/exports/pass.yml",
		"serialiseExpectedYAML": "__testdata/exports/serialise-expected.yml",
		"passJSON":              "__testdata/exports/pass.json",
		"serialiseExpectedJSON": "__testdata/exports/serialise-expected.json",
	}

	for name, filePath := range fixturesToLoad {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			c.Error(err)
			c.FailNow()
		}
		s.specFixtures[name] = specBytes
	}
}

func (s *ExportTestSuite) Test_parses_valid_string_export_yaml_input(c *C) {
	targetExport := &Export{}
	err := yaml.Unmarshal([]byte(s.specFixtures["passYAML"]), targetExport)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetExport, DeepEquals, &Export{
		Type:        ExportTypeString,
		Description: "The arn of the queue used to send order workloads to",
		Field:       "resources.orderQueue.state.arn",
	})
}

func (s *ExportTestSuite) Test_serialise_valid_export_yaml_input(c *C) {
	expected := &Export{}
	err := yaml.Unmarshal([]byte(s.specFixtures["serialiseExpectedYAML"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	serialisedBytes, err := yaml.Marshal(&Export{
		Type:        ExportTypeString,
		Description: "The ARN of the function used to save orders to the system.",
		Field:       "resources.saveOrdersFunction.state.functionArn",
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetExport := &Export{}
	err = yaml.Unmarshal(serialisedBytes, targetExport)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetExport, DeepEquals, expected)
}

func (s *ExportTestSuite) Test_parses_valid_export_json_input(c *C) {
	targetExport := &Export{}
	err := json.Unmarshal([]byte(s.specFixtures["passJSON"]), targetExport)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetExport, DeepEquals, &Export{
		Type:        ExportTypeString,
		Description: "The arn of the queue used to send order workloads to",
		Field:       "resources.orderQueue.state.arn",
	})
}

func (s *ExportTestSuite) Test_serialise_valid_export_json_input(c *C) {
	expected := &Export{}
	err := yaml.Unmarshal([]byte(s.specFixtures["serialiseExpectedJSON"]), expected)
	if err != nil {
		c.Error(fmt.Errorf("failed to parse expected fixture to compare with: %s", err.Error()))
		c.FailNow()
	}

	serialisedBytes, err := json.Marshal(&Export{
		Type:        ExportTypeString,
		Description: "The ARN of the function used to save orders to the system.",
		Field:       "resources.saveOrdersFunction.state.functionArn",
	})
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	targetExport := &Export{}
	err = json.Unmarshal(serialisedBytes, targetExport)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}

	c.Assert(targetExport, DeepEquals, expected)
}
