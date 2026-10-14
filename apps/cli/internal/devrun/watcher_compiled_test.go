package devrun

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// Returns a fixed manifest.
type stubExtractor struct {
	manifest *preprocess.HandlerManifest
	err      error
}

func (e *stubExtractor) Extract(context.Context) (*preprocess.HandlerManifest, error) {
	return e.manifest, e.err
}

// Counts container restarts and ignores everything else.
type restartRecorder struct {
	docker.RuntimeContainerManager
	restarts int
}

func (r *restartRecorder) RestartContainer(context.Context, string) error {
	r.restarts++
	return nil
}

type WatcherCompiledSuite struct {
	suite.Suite
	docker   *restartRecorder
	manifest *preprocess.HandlerManifest
}

func (s *WatcherCompiledSuite) SetupTest() {
	s.docker = &restartRecorder{}
	s.manifest = &preprocess.HandlerManifest{
		Version: "1",
		FunctionHandlers: []preprocess.FunctionHandler{
			{ResourceName: "getOrder"},
		},
	}
}

// Builds a watcher whose re-extraction returns the same handler set it started
// with, so only the rebuild distinguishes the two runtimes.
func (s *WatcherCompiledSuite) watcher(rebuild func(context.Context) error) *HandlerWatcher {
	return NewHandlerWatcher(WatcherConfig{
		AppDir:      s.T().TempDir(),
		Runtime:     "os-only.debian13",
		Extractor:   &stubExtractor{manifest: s.manifest},
		Blueprint:   &schema.Blueprint{},
		SpecFormat:  schema.YAMLSpecFormat,
		Docker:      s.docker,
		ContainerID: "container",
		Output:      NewOutput(io.Discard, false),
		Logger:      zap.NewNop(),
		Rebuild:     rebuild,
	}, s.manifest)
}

// A compiled application is running a binary, so an unchanged handler set
// still leaves the container executing the old build.
func (s *WatcherCompiledSuite) Test_rebuilds_and_restarts_even_when_the_handlers_are_unchanged() {
	rebuilt := 0
	w := s.watcher(func(context.Context) error {
		rebuilt++
		return nil
	})

	w.handleChange(context.Background())

	s.Assert().Equal(1, rebuilt)
	s.Assert().Equal(1, s.docker.restarts)
}

// An interpreted runtime picks changed source up itself, so an unchanged
// handler set is not worth a restart.
func (s *WatcherCompiledSuite) Test_without_a_rebuild_an_unchanged_handler_set_does_not_restart() {
	w := s.watcher(nil)

	w.handleChange(context.Background())

	s.Assert().Zero(s.docker.restarts)
}

// A build that no longer compiles should leave the last working one serving
// rather than take the session down with it.
func (s *WatcherCompiledSuite) Test_a_failed_rebuild_leaves_the_container_alone() {
	w := s.watcher(func(context.Context) error {
		return errors.New("go build failed")
	})

	w.handleChange(context.Background())

	s.Assert().Zero(s.docker.restarts)
}

// Nothing should be rebuilt from source that the extractor has not seen yet,
// so the rebuild has to come first.
func (s *WatcherCompiledSuite) Test_the_rebuild_runs_before_extraction() {
	var order []string
	w := s.watcher(func(context.Context) error {
		order = append(order, "rebuild")
		return nil
	})
	w.config.Extractor = &stubExtractor{manifest: s.manifest, err: errors.New("extract failed")}

	w.handleChange(context.Background())

	s.Assert().Equal([]string{"rebuild"}, order)
	s.Assert().Zero(s.docker.restarts, "a failed extraction should not restart")
}

func TestWatcherCompiledSuite(t *testing.T) { suite.Run(t, new(WatcherCompiledSuite)) }
