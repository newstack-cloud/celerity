package schema

import (
	"os"

	"github.com/two-hundred/celerity/libs/common/testhelpers"
	. "gopkg.in/check.v1"
)

type LoadTestSuite struct {
	specFixtures map[string]fixture
}

type fixture struct {
	filePath  string
	stringVal string
}

var _ = Suite(&LoadTestSuite{})

func (s *LoadTestSuite) SetUpSuite(c *C) {
	s.specFixtures = make(map[string]fixture)
	fixturesToLoad := map[string]string{
		"yaml":            "__testdata/load/blueprint.yml",
		"json":            "__testdata/load/blueprint.json",
		"yamlWithInclude": "__testdata/load/blueprint-with-include.yml",
		"jsonWithInclude": "__testdata/load/blueprint-with-include.json",
	}

	for name, filePath := range fixturesToLoad {
		specBytes, err := os.ReadFile(filePath)
		if err != nil {
			c.Error(err)
			c.FailNow()
		}
		s.specFixtures[name] = fixture{
			filePath:  filePath,
			stringVal: string(specBytes),
		}
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_yaml_file(c *C) {
	blueprint, err := Load(s.specFixtures["yaml"].filePath, YAMLSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_json_file(c *C) {
	blueprint, err := Load(s.specFixtures["json"].filePath, JSONSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_yaml_file_with_includes(c *C) {
	blueprint, err := Load(s.specFixtures["yamlWithInclude"].filePath, YAMLSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_json_file_with_include(c *C) {
	blueprint, err := Load(s.specFixtures["jsonWithInclude"].filePath, JSONSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_yaml_string(c *C) {
	blueprint, err := LoadString(s.specFixtures["yaml"].stringVal, YAMLSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}

func (s *LoadTestSuite) Test_loads_blueprint_from_json_string(c *C) {
	blueprint, err := LoadString(s.specFixtures["json"].stringVal, JSONSpecFormat)
	if err != nil {
		c.Error(err)
		c.FailNow()
	}
	err = testhelpers.Snapshot(blueprint)
	if err != nil {
		c.Error(err)
	}
}
