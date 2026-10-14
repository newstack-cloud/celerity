package build

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CompileNodejsSuite struct {
	suite.Suite
}

func TestCompileNodejsSuite(t *testing.T) {
	suite.Run(t, new(CompileNodejsSuite))
}

func (s *CompileNodejsSuite) Test_Compile_withTsconfig_invokesLocalTsc_npm() {
	stub := &stubExec{}
	compiler := &NodeSourceCompiler{Exec: stub, PM: NodePMNpm}

	srcDir := s.T().TempDir()
	outDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(
		filepath.Join(srcDir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0644,
	))

	err := compiler.Compile(context.Background(), srcDir, outDir, io.Discard)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("npm", call.name)
	s.Equal([]string{"exec", "--", "tsc", "--outDir", outDir, "--declaration", "false", "--sourceMap", "false"}, call.args)
	s.Equal(srcDir, call.dir)
}

func (s *CompileNodejsSuite) Test_Compile_withTsconfig_invokesLocalTsc_pnpm() {
	stub := &stubExec{}
	compiler := &NodeSourceCompiler{Exec: stub, PM: NodePMPnpm}

	srcDir := s.T().TempDir()
	outDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(
		filepath.Join(srcDir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0644,
	))

	err := compiler.Compile(context.Background(), srcDir, outDir, io.Discard)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("pnpm", call.name)
	s.Equal([]string{"exec", "tsc", "--outDir", outDir, "--declaration", "false", "--sourceMap", "false"}, call.args)
}

func (s *CompileNodejsSuite) Test_Compile_withTsconfig_invokesLocalTsc_yarn() {
	stub := &stubExec{}
	compiler := &NodeSourceCompiler{Exec: stub, PM: NodePMYarn}

	srcDir := s.T().TempDir()
	outDir := s.T().TempDir()

	s.Require().NoError(os.WriteFile(
		filepath.Join(srcDir, "tsconfig.json"), []byte(`{"compilerOptions":{}}`), 0644,
	))

	err := compiler.Compile(context.Background(), srcDir, outDir, io.Discard)
	s.Require().NoError(err)

	s.Require().Len(stub.calls, 1)
	call := stub.calls[0]
	s.Equal("yarn", call.name)
	s.Equal([]string{"tsc", "--outDir", outDir, "--declaration", "false", "--sourceMap", "false"}, call.args)
}

func (s *CompileNodejsSuite) Test_Compile_withoutTsconfig_skips() {
	stub := &stubExec{}
	compiler := &NodeSourceCompiler{Exec: stub, PM: NodePMNpm}

	srcDir := s.T().TempDir()
	outDir := s.T().TempDir()

	err := compiler.Compile(context.Background(), srcDir, outDir, io.Discard)
	s.NoError(err)
	s.Empty(stub.calls)
}
