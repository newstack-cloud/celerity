package build

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RegistrySuite struct {
	suite.Suite
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

// --- stub implementations ---

type stubStrategy struct{}

func (s *stubStrategy) Build(_ context.Context, _ *BuildParams) (*Manifest, error) {
	return &Manifest{}, nil
}

type stubInstaller struct{}

func (s *stubInstaller) InstallDependencies(_ context.Context, _, _ string, _ []string, _ io.Writer) error {
	return nil
}

type stubCompiler struct{}

func (s *stubCompiler) Compile(_ context.Context, _, _ string, _ io.Writer) error {
	return nil
}

type stubDepReader struct{}

func (s *stubDepReader) ReadHandlerDependencies(_ string) (map[string][]string, error) {
	return map[string][]string{"h": {"dep"}}, nil
}

// --- GetStrategy ---

func (s *RegistrySuite) Test_GetStrategy_returnsRegisteredStrategy() {
	r := NewRegistry()
	strat := &stubStrategy{}
	r.RegisterStrategy("aws-serverless", strat)

	got, err := r.GetStrategy("aws-serverless")

	s.NoError(err)
	s.Equal(strat, got)
}

func (s *RegistrySuite) Test_GetStrategy_returnsErrorForUnregisteredTarget() {
	r := NewRegistry()

	_, err := r.GetStrategy("gcp-cloud-run")

	s.Error(err)
	s.Contains(err.Error(), "gcp-cloud-run")
}

// --- GetDependencyInstaller ---

func (s *RegistrySuite) Test_GetDependencyInstaller_matchesByRuntimePrefix() {
	r := NewRegistry()
	inst := &stubInstaller{}
	r.RegisterDependencyInstaller("nodejs", inst)

	got, err := r.GetDependencyInstaller("nodejs24.x")

	s.NoError(err)
	s.Equal(inst, got)
}

func (s *RegistrySuite) Test_GetDependencyInstaller_returnsErrorForUnregisteredRuntime() {
	r := NewRegistry()

	_, err := r.GetDependencyInstaller("ruby3.3")

	s.Error(err)
	s.Contains(err.Error(), "ruby3.3")
}

// --- GetSourceCompiler ---

func (s *RegistrySuite) Test_GetSourceCompiler_returnsNilForUnregisteredRuntime() {
	r := NewRegistry()

	got := r.GetSourceCompiler("python3.12")

	s.Nil(got)
}

func (s *RegistrySuite) Test_GetSourceCompiler_returnsCompilerByPrefix() {
	r := NewRegistry()
	comp := &stubCompiler{}
	r.RegisterSourceCompiler("nodejs", comp)

	got := r.GetSourceCompiler("nodejs24.x")

	s.Equal(comp, got)
}

// --- GetHandlerDependencyReader ---

func (s *RegistrySuite) Test_GetHandlerDependencyReader_returnsNilForUnregisteredRuntime() {
	r := NewRegistry()

	got := r.GetHandlerDependencyReader("rust1.x")

	s.Nil(got)
}

func (s *RegistrySuite) Test_GetHandlerDependencyReader_returnsReaderByPrefix() {
	r := NewRegistry()
	reader := &stubDepReader{}
	r.RegisterHandlerDependencyReader("python", reader)

	got := r.GetHandlerDependencyReader("python3.12")

	s.Equal(reader, got)
}
