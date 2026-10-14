package devrun

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/newstack-cloud/celerity/apps/cli/internal/consts"
	"github.com/newstack-cloud/celerity/apps/cli/internal/docker"
	"github.com/newstack-cloud/celerity/apps/cli/internal/handlertoolchain"
	"github.com/newstack-cloud/celerity/apps/cli/internal/preprocess"
	"go.uber.org/zap"
)

const (
	watchDebounce = 500 * time.Millisecond
)

// WatcherConfig holds the dependencies for the file watcher.
type WatcherConfig struct {
	AppDir      string
	Runtime     string
	Extractor   Extractor
	Blueprint   *schema.Blueprint
	SpecFormat  schema.SpecFormat
	Docker      docker.RuntimeContainerManager
	ContainerID string
	Output      *Output
	Logger      *zap.Logger
	// Rebuild rebuilds the handlers executable, and is nil for a runtime that
	// hosts the language itself. Its presence is also what makes any source
	// change worth a restart, rather than only a changed handler set.
	Rebuild func(ctx context.Context) error

	// WatchPaths is what the resolved toolchain reports as worth watching, and
	// is nil for a runtime that hosts the language itself. It has to come from
	// the toolchain for an OS-only runtime, whose identifier names no
	// extensions and whose conventions carry none, leaving every change
	// irrelevant if they were used instead.
	WatchPaths *handlertoolchain.WatchPaths
}

// Extractor reports the handlers an application serves.
//
// An interface rather than *preprocess.Extractor because for an OS-only
// runtime extraction belongs to whichever toolchain built the executable, and
// the dev session has one of those instead.
type Extractor interface {
	Extract(ctx context.Context) (*preprocess.HandlerManifest, error)
}

// HandlerWatcher watches the source directory for handler changes
// and restarts the container when structural changes are detected.
type HandlerWatcher struct {
	config       WatcherConfig
	lastManifest *preprocess.HandlerManifest
}

// NewHandlerWatcher creates a new file watcher.
func NewHandlerWatcher(config WatcherConfig, initialManifest *preprocess.HandlerManifest) *HandlerWatcher {
	return &HandlerWatcher{
		config:       config,
		lastManifest: initialManifest,
	}
}

// Watch starts watching the source directory for file changes.
// Blocks until the context is cancelled.
func (w *HandlerWatcher) Watch(ctx context.Context) error {
	paths, ok := w.watchPaths()
	if !ok {
		w.config.Logger.Debug("nothing says what to watch, skipping watcher",
			zap.String("runtime", w.config.Runtime),
		)
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	watched := 0
	for _, root := range paths.Roots {
		dir := filepath.Join(w.config.AppDir, root)
		if err := addWatchRecursive(watcher, dir, paths.SkipDirs); err != nil {
			w.config.Logger.Debug("cannot watch directory",
				zap.String("dir", dir),
				zap.Error(err),
			)
			continue
		}
		watched++
	}
	if watched == 0 {
		return nil
	}

	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isRelevantChange(event, paths) {
				continue
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(watchDebounce, func() {
				w.handleChange(ctx)
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.config.Logger.Warn("file watcher error", zap.Error(err))
		}
	}
}

func (w *HandlerWatcher) handleChange(ctx context.Context) {
	// Rebuilt before anything else, and the container left alone when it
	// fails. A compiled application that no longer compiles should leave the
	// last working build serving rather than take the session down.
	if w.config.Rebuild != nil {
		w.config.Output.PrintInfo("[watcher] Change detected, rebuilding...")
		if err := w.config.Rebuild(ctx); err != nil {
			w.config.Output.PrintWarning("Rebuild failed, leaving the running build in place", err)
			return
		}
	}

	manifest, err := w.config.Extractor.Extract(ctx)
	if err != nil {
		w.config.Logger.Warn("re-extraction failed", zap.Error(err))
		return
	}

	handlersChanged := !w.lastManifest.Equal(manifest)

	// An interpreted runtime picks up changed source on its own, so only a
	// changed handler set is worth a restart. A rebuilt executable is a new
	// file the running container is not executing, so it always is.
	if !handlersChanged && w.config.Rebuild == nil {
		return
	}

	if handlersChanged {
		merged, err := preprocess.Merge(w.config.Blueprint, manifest, w.config.Logger)
		if err != nil {
			w.config.Logger.Warn("re-merge failed", zap.Error(err))
			return
		}

		outputDir := filepath.Join(w.config.AppDir, ".celerity")
		if _, err := preprocess.WriteMerged(merged, w.config.SpecFormat, outputDir); err != nil {
			w.config.Logger.Warn("writing merged blueprint failed", zap.Error(err))
			return
		}
		w.config.Blueprint = merged
	}

	if err := w.config.Docker.RestartContainer(ctx, w.config.ContainerID); err != nil {
		w.config.Logger.Warn("container restart failed", zap.Error(err))
		return
	}

	w.lastManifest = manifest
	w.config.Output.PrintStep("Container restarted")
}

// The toolchain paths are preferred over the runtime conventions. An OS-only
// runtime identifier names a base operating system and nothing about what
// compiled the executable, so its conventions carry no extensions and a
// session resolved from them treats every change as irrelevant.
func (w *HandlerWatcher) watchPaths() (handlertoolchain.WatchPaths, bool) {
	if w.config.WatchPaths != nil {
		paths := *w.config.WatchPaths
		if len(paths.Roots) > 0 && len(paths.Extensions) > 0 {
			return paths, true
		}
	}

	conv, ok := consts.ConventionsForRuntime(w.config.Runtime)
	if !ok || len(conv.WatchExtensions) == 0 {
		return handlertoolchain.WatchPaths{}, false
	}

	roots := conv.WatchRoots
	if len(roots) == 0 {
		roots = []string{"src"}
	}
	// SkipFiles is left empty deliberately. It exists so a toolchain can skip
	// the file its own Prepare rewrites, and a runtime that hosts the language
	// itself has no such file, so there is nothing for a convention to name.
	return handlertoolchain.WatchPaths{
		Roots:      roots,
		Extensions: conv.WatchExtensions,
		SkipDirs:   conv.WatchSkipDirs,
	}, true
}

func isRelevantChange(event fsnotify.Event, paths handlertoolchain.WatchPaths) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	if slices.Contains(paths.SkipFiles, filepath.Base(event.Name)) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(event.Name))
	return slices.Contains(paths.Extensions, ext)
}

func addWatchRecursive(watcher *fsnotify.Watcher, dir string, skipDirs []string) error {
	skipSet := map[string]struct{}{".git": {}}
	for _, d := range skipDirs {
		skipSet[d] = struct{}{}
	}

	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipSet[d.Name()]; skip {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}
