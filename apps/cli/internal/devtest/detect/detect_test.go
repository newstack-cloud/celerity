package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type DetectTestSuite struct {
	suite.Suite
}

func TestDetectTestSuite(t *testing.T) {
	suite.Run(t, new(DetectTestSuite))
}

// --- LoadPackageJSON ---

func (s *DetectTestSuite) Test_load_package_json() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(
		filepath.Join(dir, "package.json"),
		[]byte(`{"type":"module","packageManager":"yarn@4.0.0","scripts":{"test":"vitest"},"devDependencies":{"vitest":"^1.0.0"}}`),
		0o644,
	))

	pkg, err := LoadPackageJSON(dir)
	s.Require().NoError(err)
	s.Assert().Equal("module", pkg.Type)
	s.Assert().True(pkg.IsESM())
	s.Assert().Equal("yarn@4.0.0", pkg.PackageManager)
	s.Assert().Equal("vitest", pkg.Scripts["test"])
}

func (s *DetectTestSuite) Test_load_package_json_not_found() {
	dir := s.T().TempDir()
	_, err := LoadPackageJSON(dir)
	s.Assert().Error(err)
}

func (s *DetectTestSuite) Test_load_package_json_invalid() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{invalid}`), 0o644))
	_, err := LoadPackageJSON(dir)
	s.Assert().Error(err)
}

func (s *DetectTestSuite) Test_is_esm_false_for_commonjs() {
	pkg := &PackageJSON{Type: "commonjs"}
	s.Assert().False(pkg.IsESM())
}

func (s *DetectTestSuite) Test_is_esm_false_when_empty() {
	pkg := &PackageJSON{}
	s.Assert().False(pkg.IsESM())
}

// --- DetectPackageManager ---

func (s *DetectTestSuite) Test_detect_pm_from_package_manager_field_yarn() {
	pkg := &PackageJSON{PackageManager: "yarn@4.0.0"}
	s.Assert().Equal("yarn", DetectPackageManager(pkg, s.T().TempDir()))
}

func (s *DetectTestSuite) Test_detect_pm_from_package_manager_field_pnpm() {
	pkg := &PackageJSON{PackageManager: "pnpm@9.0.0"}
	s.Assert().Equal("pnpm", DetectPackageManager(pkg, s.T().TempDir()))
}

func (s *DetectTestSuite) Test_detect_pm_from_package_manager_field_npm() {
	pkg := &PackageJSON{PackageManager: "npm@10.0.0"}
	s.Assert().Equal("npm", DetectPackageManager(pkg, s.T().TempDir()))
}

func (s *DetectTestSuite) Test_detect_pm_ignores_unknown_manager() {
	pkg := &PackageJSON{PackageManager: "bun@1.0.0"}
	dir := s.T().TempDir()
	// No lock files, so falls through to npm default.
	s.Assert().Equal("npm", DetectPackageManager(pkg, dir))
}

func (s *DetectTestSuite) Test_detect_pm_from_yarn_lock() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte(""), 0o644))
	s.Assert().Equal("yarn", DetectPackageManager(&PackageJSON{}, dir))
}

func (s *DetectTestSuite) Test_detect_pm_from_pnpm_lock() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte(""), 0o644))
	s.Assert().Equal("pnpm", DetectPackageManager(&PackageJSON{}, dir))
}

func (s *DetectTestSuite) Test_detect_pm_from_package_lock() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(""), 0o644))
	s.Assert().Equal("npm", DetectPackageManager(&PackageJSON{}, dir))
}

func (s *DetectTestSuite) Test_detect_pm_defaults_to_npm() {
	s.Assert().Equal("npm", DetectPackageManager(&PackageJSON{}, s.T().TempDir()))
}

// --- DetectFramework ---

func (s *DetectTestSuite) Test_detect_framework_vitest_in_dev_deps() {
	pkg := &PackageJSON{DevDependencies: map[string]string{"vitest": "^1.0.0"}}
	s.Assert().Equal("vitest", DetectFramework(pkg))
}

func (s *DetectTestSuite) Test_detect_framework_jest_in_deps() {
	pkg := &PackageJSON{Dependencies: map[string]string{"jest": "^29.0.0"}}
	s.Assert().Equal("jest", DetectFramework(pkg))
}

func (s *DetectTestSuite) Test_detect_framework_ava() {
	pkg := &PackageJSON{DevDependencies: map[string]string{"ava": "^6.0.0"}}
	s.Assert().Equal("ava", DetectFramework(pkg))
}

func (s *DetectTestSuite) Test_detect_framework_vitest_takes_priority() {
	pkg := &PackageJSON{DevDependencies: map[string]string{"vitest": "^1.0.0", "jest": "^29.0.0"}}
	s.Assert().Equal("vitest", DetectFramework(pkg))
}

func (s *DetectTestSuite) Test_detect_framework_none_found() {
	pkg := &PackageJSON{DevDependencies: map[string]string{"typescript": "^5.0.0"}}
	s.Assert().Equal("", DetectFramework(pkg))
}

func (s *DetectTestSuite) Test_detect_framework_empty_deps() {
	s.Assert().Equal("", DetectFramework(&PackageJSON{}))
}

// --- FrameworkCommand ---

func (s *DetectTestSuite) Test_framework_command_vitest_yarn() {
	args := FrameworkCommand("yarn", "vitest", []string{"src"}, false)
	s.Assert().Equal([]string{"yarn", "vitest", "run", "src"}, args)
}

func (s *DetectTestSuite) Test_framework_command_vitest_npm() {
	args := FrameworkCommand("npm", "vitest", []string{"src"}, false)
	s.Assert().Equal([]string{"npm", "exec", "vitest", "run", "src"}, args)
}

func (s *DetectTestSuite) Test_framework_command_vitest_pnpm() {
	args := FrameworkCommand("pnpm", "vitest", []string{"src"}, false)
	s.Assert().Equal([]string{"pnpm", "exec", "vitest", "run", "src"}, args)
}

func (s *DetectTestSuite) Test_framework_command_vitest_with_coverage() {
	args := FrameworkCommand("yarn", "vitest", []string{"src"}, true)
	s.Assert().Contains(args, "--coverage")
}

func (s *DetectTestSuite) Test_framework_command_vitest_multiple_dirs() {
	args := FrameworkCommand("npm", "vitest", []string{"src", "tests/integration"}, false)
	s.Assert().Equal([]string{"npm", "exec", "vitest", "run", "src", "tests/integration"}, args)
}

func (s *DetectTestSuite) Test_framework_command_jest_npm() {
	args := FrameworkCommand("npm", "jest", []string{"src"}, false)
	s.Assert().Equal([]string{"npm", "exec", "jest", "--roots", "src"}, args)
}

func (s *DetectTestSuite) Test_framework_command_jest_yarn() {
	args := FrameworkCommand("yarn", "jest", []string{"src"}, false)
	s.Assert().Equal([]string{"yarn", "jest", "--roots", "src"}, args)
}

func (s *DetectTestSuite) Test_framework_command_jest_with_coverage() {
	args := FrameworkCommand("npm", "jest", []string{"src"}, true)
	s.Assert().Contains(args, "--coverage")
}

func (s *DetectTestSuite) Test_framework_command_ava_npm() {
	args := FrameworkCommand("npm", "ava", []string{"src", "tests"}, false)
	s.Assert().Equal([]string{"npm", "exec", "ava", "src/**/*.test.{ts,js}", "tests/**/*.test.{ts,js}"}, args)
}

func (s *DetectTestSuite) Test_framework_command_ava_pnpm() {
	args := FrameworkCommand("pnpm", "ava", []string{"src"}, false)
	s.Assert().Equal([]string{"pnpm", "exec", "ava", "src/**/*.test.{ts,js}"}, args)
}

func (s *DetectTestSuite) Test_framework_command_unknown_falls_back_to_pm_test() {
	args := FrameworkCommand("yarn", "unknown", nil, false)
	s.Assert().Equal([]string{"yarn", "test"}, args)
}

// --- FindPytest ---

func (s *DetectTestSuite) Test_find_pytest_in_dot_venv() {
	dir := s.T().TempDir()
	binDir := filepath.Join(dir, ".venv", "bin")
	s.Require().NoError(os.MkdirAll(binDir, 0o755))
	s.Require().NoError(os.WriteFile(filepath.Join(binDir, "pytest"), []byte(""), 0o755))

	s.Assert().Equal(filepath.Join(binDir, "pytest"), FindPytest(dir))
}

func (s *DetectTestSuite) Test_find_pytest_in_venv() {
	dir := s.T().TempDir()
	binDir := filepath.Join(dir, "venv", "bin")
	s.Require().NoError(os.MkdirAll(binDir, 0o755))
	s.Require().NoError(os.WriteFile(filepath.Join(binDir, "pytest"), []byte(""), 0o755))

	s.Assert().Equal(filepath.Join(binDir, "pytest"), FindPytest(dir))
}

func (s *DetectTestSuite) Test_find_pytest_not_found() {
	dir := s.T().TempDir()
	s.Assert().Equal("", FindPytest(dir))
}

// --- PytestArgs ---

func (s *DetectTestSuite) Test_pytest_args_basic() {
	args := PytestArgs("/path/pytest", []string{"tests/unit"}, false)
	s.Assert().Equal([]string{"/path/pytest", "tests/unit", "-v"}, args)
}

func (s *DetectTestSuite) Test_pytest_args_with_coverage() {
	args := PytestArgs("/path/pytest", []string{"tests"}, true)
	s.Assert().Contains(args, "--cov")
	s.Assert().Contains(args, "--cov-report=term-missing")
}

func (s *DetectTestSuite) Test_pytest_args_no_dirs() {
	args := PytestArgs("pytest", nil, false)
	s.Assert().Equal([]string{"pytest", "-v"}, args)
}

// --- FilterExistingDirs ---

func (s *DetectTestSuite) Test_filter_existing_dirs() {
	dir := s.T().TempDir()
	s.Require().NoError(os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	s.Require().NoError(os.MkdirAll(filepath.Join(dir, "tests"), 0o755))

	result := FilterExistingDirs(dir, []string{"src", "tests", "missing"})
	s.Assert().Equal([]string{"src", "tests"}, result)
}

func (s *DetectTestSuite) Test_filter_existing_dirs_none_exist() {
	dir := s.T().TempDir()
	result := FilterExistingDirs(dir, []string{"a", "b"})
	s.Assert().Empty(result)
}

// --- ExpandSuites ---

func (s *DetectTestSuite) Test_expand_suites_deduplicates() {
	result := ExpandSuites([]string{"unit", "unit", "integration"})
	s.Assert().Equal([]string{"unit", "integration"}, result)
}

func (s *DetectTestSuite) Test_expand_suites_preserves_order() {
	result := ExpandSuites([]string{"integration", "unit"})
	s.Assert().Equal([]string{"integration", "unit"}, result)
}

func (s *DetectTestSuite) Test_expand_suites_empty() {
	result := ExpandSuites([]string(nil))
	s.Assert().Empty(result)
}
