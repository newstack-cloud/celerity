package preprocess

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ExtractorTestSuite struct {
	suite.Suite
	logger *zap.Logger
}

func (s *ExtractorTestSuite) SetupTest() {
	logger, _ := zap.NewDevelopment()
	s.logger = logger
}

// Lays out the parts of node_modules that buildNodeArgs checks for, so a test
// can omit one and assert on the resulting error.
func (s *ExtractorTestSuite) projectRoot(withExtractBin, withLoader bool) string {
	root := s.T().TempDir()

	if withExtractBin {
		binDir := filepath.Join(root, "node_modules", ".bin")
		s.Require().NoError(os.MkdirAll(binDir, 0755))
		s.Require().NoError(
			os.WriteFile(filepath.Join(binDir, "celerity-extract"), []byte("#!/bin/sh\n"), 0755),
		)
	}

	if withLoader {
		s.Require().NoError(
			os.MkdirAll(filepath.Join(root, "node_modules", "@swc-node", "register"), 0755),
		)
	}

	return root
}

func (s *ExtractorTestSuite) newExtractor(root string) *Extractor {
	return NewExtractor(ExtractorConfig{
		Runtime:     "nodejs22.x",
		ModulePath:  "src/app-module.ts",
		ProjectRoot: root,
	}, s.logger)
}

// The loader has to be an SWC one: esbuild does not implement
// emitDecoratorMetadata, and without design:paramtypes a handler that injects a
// service by type alone contributes no edge to the injection graph, so its
// resource links are silently lost.
func (s *ExtractorTestSuite) Test_buildNodeArgs_uses_the_swc_loader() {
	root := s.projectRoot(true, true)

	args, err := s.newExtractor(root).buildNodeArgs()

	s.Require().NoError(err)
	s.Require().Equal([]string{
		"node",
		"--import", "@swc-node/register/esm-register",
		filepath.Join(root, "node_modules", ".bin", "celerity-extract"),
		"--module", "src/app-module.ts",
		"--project-root", root,
	}, args)
	s.Assert().NotContains(args, "tsx")
}

func (s *ExtractorTestSuite) Test_buildNodeArgs_reports_a_missing_loader() {
	root := s.projectRoot(true, false)

	_, err := s.newExtractor(root).buildNodeArgs()

	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "@swc-node/register")
}

func (s *ExtractorTestSuite) Test_buildNodeArgs_reports_a_missing_extraction_cli() {
	root := s.projectRoot(false, true)

	_, err := s.newExtractor(root).buildNodeArgs()

	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "@celerity-sdk/cli")
}

func TestExtractorTestSuite(t *testing.T) {
	suite.Run(t, new(ExtractorTestSuite))
}
