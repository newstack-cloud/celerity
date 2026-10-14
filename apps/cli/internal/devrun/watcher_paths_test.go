package devrun

import (
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// What to watch is a property of the language rather than of the runtime, and
// an OS-only runtime identifier names only a base operating system. So the
// conventions for one carry no extensions at all, and a watcher resolving
// from them finds every change irrelevant and never reloads.
type WatcherPathsSuite struct {
	suite.Suite
}

func TestWatcherPathsSuite(t *testing.T) { suite.Run(t, new(WatcherPathsSuite)) }

func (s *WatcherPathsSuite) watcher(runtime string, paths *handlertoolchain.WatchPaths) *HandlerWatcher {
	return NewHandlerWatcher(WatcherConfig{
		AppDir:     s.T().TempDir(),
		Runtime:    runtime,
		Logger:     zap.NewNop(),
		WatchPaths: paths,
	}, nil)
}

// A change of the kind a saved file produces.
func write(name string) fsnotify.Event {
	return fsnotify.Event{Name: name, Op: fsnotify.Write}
}

func (s *WatcherPathsSuite) Test_an_os_only_runtime_watches_what_its_toolchain_names() {
	w := s.watcher("os-only.debian13", &handlertoolchain.WatchPaths{
		Roots:      []string{"."},
		Extensions: []string{".go"},
	})

	paths, ok := w.watchPaths()

	s.Require().True(ok)
	s.Assert().Equal([]string{"."}, paths.Roots)
	s.Assert().True(isRelevantChange(write("cmd/app/orders.go"), paths))
}

// The file Prepare writes is a source file in a watched root, so reacting to
// it would have each rebuild trigger the next for as long as the session ran.
func (s *WatcherPathsSuite) Test_a_change_to_the_generated_platform_file_is_not_relevant() {
	w := s.watcher("os-only.debian13", &handlertoolchain.WatchPaths{
		Roots:      []string{"."},
		Extensions: []string{".go"},
		SkipFiles:  []string{"celerity_platform_gen.go"},
	})

	paths, ok := w.watchPaths()

	s.Require().True(ok)
	s.Assert().False(isRelevantChange(write("celerity_platform_gen.go"), paths))
	s.Assert().True(isRelevantChange(write("handlers.go"), paths),
		"only that one file, not every generated-looking name")
}

// Reported rather than watched with nothing, because a watcher registered on
// the module root that treats no extension as relevant is running and reloading
// nothing.
func (s *WatcherPathsSuite) Test_an_os_only_runtime_with_no_toolchain_has_nothing_to_watch() {
	w := s.watcher("os-only.debian13", nil)

	_, ok := w.watchPaths()

	s.Assert().False(ok)
}

func (s *WatcherPathsSuite) Test_a_runtime_that_hosts_the_language_still_uses_its_conventions() {
	w := s.watcher("nodejs24.x", nil)

	paths, ok := w.watchPaths()

	s.Require().True(ok)
	s.Assert().Equal([]string{"src"}, paths.Roots)
	s.Assert().True(isRelevantChange(write("src/handlers.ts"), paths))
	s.Assert().False(isRelevantChange(write("src/notes.md"), paths))
}

// A toolchain that names roots but no extensions would otherwise watch and
// react to nothing, which is the failure this guards against rather than a
// shape any toolchain is expected to return.
func (s *WatcherPathsSuite) Test_toolchain_paths_missing_extensions_fall_back_to_the_conventions() {
	w := s.watcher("nodejs24.x", &handlertoolchain.WatchPaths{Roots: []string{"."}})

	paths, ok := w.watchPaths()

	s.Require().True(ok)
	s.Assert().Equal([]string{"src"}, paths.Roots)
}

func (s *WatcherPathsSuite) Test_a_change_that_is_not_a_write_is_not_relevant() {
	paths := handlertoolchain.WatchPaths{Extensions: []string{".go"}}

	s.Assert().False(isRelevantChange(fsnotify.Event{Name: "orders.go", Op: fsnotify.Chmod}, paths))
}
