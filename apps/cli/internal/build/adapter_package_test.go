package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AdapterPackageTestSuite struct {
	suite.Suite
}

func (s *AdapterPackageTestSuite) writeManifest(name, content string) string {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	return dir
}

// The check that existed before this accepted any @celerity-sdk/* package, so an
// app depending on several of them passed while missing the one the entry file
// imports. That is how a project reached a real deployment with every handler
// failing to start on a module resolution error.
func (s *AdapterPackageTestSuite) Test_other_celerity_packages_do_not_satisfy_the_adapter() {
	dir := s.writeManifest("package.json", `{
		"dependencies": {
			"@celerity-sdk/bucket": "^0.8.4",
			"@celerity-sdk/datastore": "^0.8.4",
			"@celerity-sdk/sql-database": "^0.8.4"
		},
		"devDependencies": {"@celerity-sdk/cli": "^0.8.4"}
	}`)

	declared, err := appDeclaresPackage("nodejs22.x", dir, "@celerity-sdk/serverless-aws")

	s.Require().NoError(err)
	s.False(declared)
}

func (s *AdapterPackageTestSuite) Test_adapter_declared_as_a_dependency() {
	dir := s.writeManifest("package.json", `{
		"dependencies": {"@celerity-sdk/serverless-aws": "^0.8.4"}
	}`)

	declared, err := appDeclaresPackage("nodejs22.x", dir, "@celerity-sdk/serverless-aws")

	s.Require().NoError(err)
	s.True(declared)
}

// The runtime container provides the SDK packages as peers in some setups, so a
// manifest naming the package there has declared it.
func (s *AdapterPackageTestSuite) Test_adapter_declared_as_a_peer_or_dev_dependency() {
	for name, manifest := range map[string]string{
		"peer": `{"peerDependencies": {"@celerity-sdk/serverless-aws": "^0.8.4"}}`,
		"dev":  `{"devDependencies": {"@celerity-sdk/serverless-aws": "^0.8.4"}}`,
	} {
		dir := s.writeManifest("package.json", manifest)

		declared, err := appDeclaresPackage("nodejs22.x", dir, "@celerity-sdk/serverless-aws")

		s.Require().NoError(err, name)
		s.True(declared, "%s dependency should count as declared", name)
	}
}

func (s *AdapterPackageTestSuite) Test_python_distribution_declared() {
	dir := s.writeManifest("pyproject.toml", `
[project]
dependencies = ["celerity-sdk[serverless]>=0.3.0"]
`)

	declared, err := appDeclaresPackage("python3.13", dir, "celerity-sdk")

	s.Require().NoError(err)
	s.True(declared)
}

// A commented-out dependency is not a dependency; the bundle is built from what
// the manifest actually declares.
func (s *AdapterPackageTestSuite) Test_python_commented_dependency_does_not_count() {
	dir := s.writeManifest("pyproject.toml", `
[project]
# dependencies = ["celerity-sdk"]
dependencies = ["boto3"]
`)

	declared, err := appDeclaresPackage("python3.13", dir, "celerity-sdk")

	s.Require().NoError(err)
	s.False(declared)
}

func (s *AdapterPackageTestSuite) Test_missing_manifest_is_reported_as_an_error() {
	_, err := appDeclaresPackage("nodejs22.x", s.T().TempDir(), "@celerity-sdk/serverless-aws")

	s.Require().Error(err)
}

// requireAdapterPackage builds the message a developer acts on, so it has to name
// the package rather than describe the category it belongs to.
func (s *AdapterPackageTestSuite) Test_error_names_the_missing_package() {
	dir := s.writeManifest("package.json", `{"dependencies": {"@celerity-sdk/core": "^0.8.4"}}`)

	err := requireAdapterPackage(
		&LambdaNodeBootstrap{AppDir: dir},
		"nodejs22.x",
		dir,
	)

	s.Require().Error(err)
	s.Contains(err.Error(), "@celerity-sdk/serverless-aws")
}

func (s *AdapterPackageTestSuite) Test_no_error_when_the_adapter_is_present() {
	dir := s.writeManifest("package.json",
		`{"dependencies": {"@celerity-sdk/serverless-aws": "^0.8.4"}}`)

	err := requireAdapterPackage(&LambdaNodeBootstrap{AppDir: dir}, "nodejs22.x", dir)

	s.Require().NoError(err)
}

// An unreadable manifest is not proof the dependency is absent, and failing the
// build on it would be worse than the check not running.
func (s *AdapterPackageTestSuite) Test_unreadable_manifest_does_not_fail_the_build() {
	err := requireAdapterPackage(
		&LambdaNodeBootstrap{AppDir: s.T().TempDir()},
		"nodejs22.x",
		s.T().TempDir(),
	)

	s.Require().NoError(err)
}

func TestAdapterPackageTestSuite(t *testing.T) {
	suite.Run(t, new(AdapterPackageTestSuite))
}
