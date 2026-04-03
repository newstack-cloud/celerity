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
	Extractor   *preprocess.Extractor
	Blueprint   *schema.Blueprint
	SpecFormat  schema.SpecFormat
	Docker      docker.RuntimeContainerManager
	ContainerID string
	Output      *Output
	Logger      *zap.Logger
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
	conv, ok := consts.ConventionsForRuntime(w.config.Runtime)
	if !ok {
		w.config.Logger.Debug("no conventions for runtime, skipping watcher",
			zap.String("runtime", w.config.Runtime),
		)
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	srcDir := filepath.Join(w.config.AppDir, "src")
	if err := addWatchRecursive(watcher, srcDir, conv.WatchSkipDirs); err != nil {
		w.config.Logger.Debug("cannot watch src directory", zap.Error(err))
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
			if !isRelevantChange(event, conv.WatchExtensions) {
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
	manifest, err := w.config.Extractor.Extract(ctx)
	if err != nil {
		w.config.Logger.Warn("re-extraction failed", zap.Error(err))
		return
	}

	if w.lastManifest.Equal(manifest) {
		return
	}

	w.config.Output.PrintInfo("[watcher] Handler change detected, restarting container...")

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

	if err := w.config.Docker.RestartContainer(ctx, w.config.ContainerID); err != nil {
		w.config.Logger.Warn("container restart failed", zap.Error(err))
		return
	}

	w.lastManifest = manifest
	w.config.Blueprint = merged
	w.config.Output.PrintStep("Container restarted")
}

func isRelevantChange(event fsnotify.Event, watchExtensions []string) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	ext := strings.ToLower(filepath.Ext(event.Name))
	return slices.Contains(watchExtensions, ext)
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
