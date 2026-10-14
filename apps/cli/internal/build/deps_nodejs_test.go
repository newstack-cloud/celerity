package build

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DepsNodejsSuite struct {
	suite.Suite
}

func TestDepsNodejsSuite(t *testing.T) {
	suite.Run(t, new(DepsNodejsSuite))
}

// --- filterDependencies ---

func (s *DepsNodejsSuite) Test_filterDependencies_filtersCorrectDeps() {
	pkgJSON := map[string]any{
		"dependencies": map[string]any{
			"express": "^4.18.0",
			"lodash":  "^4.17.0",
			"zod":     "^3.0.0",
		},
	}
	result := filterDependencies(pkgJSON, []string{"express", "zod"})
	s.Equal(map[string]string{"express": "^4.18.0", "zod": "^3.0.0"}, result)
}

func (s *DepsNodejsSuite) Test_filterDependencies_handlesMissingDeps() {
	pkgJSON := map[string]any{
		"dependencies": map[string]any{
			"express": "^4.18.0",
		},
	}
	result := filterDependencies(pkgJSON, []string{"express", "nonexistent"})
	s.Equal(map[string]string{"express": "^4.18.0"}, result)
}

func (s *DepsNodejsSuite) Test_filterDependencies_emptyPackageList() {
	pkgJSON := map[string]any{
		"dependencies": map[string]any{
			"express": "^4.18.0",
		},
	}
	result := filterDependencies(pkgJSON, []string{})
	s.Empty(result)
}

func (s *DepsNodejsSuite) Test_filterDependencies_noDependenciesSection() {
	pkgJSON := map[string]any{
		"name": "test-app",
	}
	result := filterDependencies(pkgJSON, []string{"express"})
	s.Empty(result)
}

// --- installAll with stub executor ---

func (s *DepsNodejsSuite) Test_installAll_copiesPackageFilesAndRunsNpmCI() {
	stub := &stubExec{}
	installer := &NodeDependencyInstaller{Exec: stub, PM: NodePMNpm}

	appDir := s.T().TempDir()
	targetDir := s.T().TempDir()

	s.writeJSON(filepath.Join(appDir, "package.json"), map[string]any{
		"name": "test", "version": "1.0.0", "dependencies": map[string]any{},
	})
	s.Require().NoError(os.WriteFile(
		filepath.Join(appDir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644,
	))

	err := installer.InstallDependencies(context.Background(), appDir, targetDir, nil, io.Discard)
	s.Require().NoError(err)

	s.FileExists(filepath.Join(targetDir, "package.json"))
	s.FileExists(filepath.Join(targetDir, "package-lock.json"))

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("npm", call.name)
	s.Equal([]string{"ci", "--omit=dev"}, call.args)
	s.Equal(targetDir, call.dir)
}

// --- installSubset with stub executor ---

func (s *DepsNodejsSuite) Test_installSubset_createsFilteredPackageJSON() {
	stub := &stubExec{}
	installer := &NodeDependencyInstaller{Exec: stub, PM: NodePMNpm}

	appDir := s.T().TempDir()
	targetDir := s.T().TempDir()

	s.writeJSON(filepath.Join(appDir, "package.json"), map[string]any{
		"name":    "test",
		"version": "1.0.0",
		"dependencies": map[string]any{
			"express": "^4.18.0",
			"lodash":  "^4.17.0",
			"zod":     "^3.0.0",
		},
	})
	s.Require().NoError(os.WriteFile(
		filepath.Join(appDir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644,
	))

	err := installer.InstallDependencies(
		context.Background(), appDir, targetDir, []string{"express", "zod"}, io.Discard,
	)
	s.Require().NoError(err)

	data, err := os.ReadFile(filepath.Join(targetDir, "package.json"))
	s.Require().NoError(err)

	var pkg map[string]any
	s.Require().NoError(json.Unmarshal(data, &pkg))

	deps := pkg["dependencies"].(map[string]any)
	s.Equal("^4.18.0", deps["express"])
	s.Equal("^3.0.0", deps["zod"])
	s.NotContains(deps, "lodash")

	s.FileExists(filepath.Join(targetDir, "package-lock.json"))

	s.Require().Len(stub.calls, 1)
	s.Equal("npm", stub.calls[0].name)
}

// --- helpers ---

func (s *DepsNodejsSuite) writeJSON(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(path, data, 0644))
}
