package handlertoolchain

import (
	"fmt"
	"maps"
	"slices"
)

// Config is the handlerBuild block of an application's deploy config.
//
// It is what makes a community SDK for a compiled language usable without any
// CLI code. The author states how to produce a handlers executable and, where
// their SDK can, how to report what it serves, and everything the CLI does
// with the result is language-neutral.
//
// It lives on the deploy config rather than in celerity.config.toml because
// that provider carries only scalars, and because a build command legitimately
// differs per deploy target.
type Config struct {
	// Command builds the handlers executable. Run through the shell, from the
	// application directory.
	Command string `json:"command"`
	// Binary is where Command leaves the executable, relative to the
	// application directory. The CLI copies it to wherever it is needed.
	Binary string `json:"binary"`
	// Env is added to Command's environment, on top of the CELERITY_BUILD_*
	// variables the CLI injects.
	Env map[string]string `json:"env,omitempty"`
	// Tidy, where set, is run after Command to reconcile the project's
	// dependency manifest.
	Tidy string `json:"tidy,omitempty"`
	// Extract reports the handlers the executable serves. Optional, and
	// without it the blueprint's hand-authored handler resources are used as
	// they are.
	Extract *ExtractConfig `json:"extract,omitempty"`
	// Test maps a suite name (unit, integration, api) to its command.
	Test map[string]string `json:"test,omitempty"`
	// WatchRoots are the directories a dev session watches, relative to the
	// application directory. Defaults to the application directory itself.
	WatchRoots []string `json:"watchRoots,omitempty"`
	// WatchExtensions are the file extensions a dev session rebuilds for.
	WatchExtensions []string `json:"watchExtensions,omitempty"`
	// WatchSkipDirs are directory names the watcher does not descend into.
	WatchSkipDirs []string `json:"watchSkipDirs,omitempty"`
	// Targets overrides any of the above per deploy target. A cross-compiled
	// binary usually lands somewhere different for each.
	Targets map[string]*Config `json:"targets,omitempty"`
}

// ExtractConfig describes a command that reports an application's handlers.
type ExtractConfig struct {
	// Command produces a handler manifest. Run through the shell, from the
	// application directory.
	Command string `json:"command"`
	// ManifestFile is where Command writes the manifest, relative to the
	// application directory. When empty the manifest is read from the
	// command's standard output.
	ManifestFile string `json:"manifestFile,omitempty"`
}

// Validate reports whether the block is usable.
func (c *Config) Validate() error {
	if c == nil {
		return nil
	}
	if c.Command == "" {
		return fmt.Errorf("handlerBuild.command is required: it is the command that builds the handlers executable")
	}
	if c.Binary == "" {
		return fmt.Errorf("handlerBuild.binary is required: it is where handlerBuild.command leaves the executable")
	}
	if c.Extract != nil && c.Extract.Command == "" {
		return fmt.Errorf("handlerBuild.extract.command is required when handlerBuild.extract is set")
	}
	for target, override := range c.Targets {
		if override == nil {
			continue
		}
		if len(override.Targets) > 0 {
			return fmt.Errorf("handlerBuild.targets.%s.targets is not allowed: overrides do not nest", target)
		}
	}
	return nil
}

// Overrides are applied field by field rather than wholesale, so an override
// naming only the binary path keeps the base command rather than silently
// blanking it.
func (c *Config) forTarget(deployTarget string) *Config {
	if c == nil {
		return nil
	}
	override, ok := c.Targets[deployTarget]
	if !ok || override == nil {
		return c
	}

	merged := *c
	merged.Targets = nil

	if override.Command != "" {
		merged.Command = override.Command
	}
	if override.Binary != "" {
		merged.Binary = override.Binary
	}
	if override.Tidy != "" {
		merged.Tidy = override.Tidy
	}
	if override.Extract != nil {
		merged.Extract = override.Extract
	}
	if len(override.Test) > 0 {
		merged.Test = override.Test
	}
	if len(override.WatchRoots) > 0 {
		merged.WatchRoots = override.WatchRoots
	}
	if len(override.WatchExtensions) > 0 {
		merged.WatchExtensions = override.WatchExtensions
	}
	if len(override.WatchSkipDirs) > 0 {
		merged.WatchSkipDirs = override.WatchSkipDirs
	}
	// Merged rather than replaced: env vars are independent settings, and an
	// override adding one should not drop the rest.
	if len(override.Env) > 0 {
		merged.Env = make(map[string]string, len(c.Env)+len(override.Env))
		maps.Copy(merged.Env, c.Env)
		maps.Copy(merged.Env, override.Env)
	} else {
		merged.Env = maps.Clone(c.Env)
	}

	return &merged
}

func (c *Config) watchPaths() WatchPaths {
	roots := c.WatchRoots
	if len(roots) == 0 {
		roots = []string{"."}
	}
	skip := c.WatchSkipDirs
	if len(skip) == 0 {
		skip = []string{".celerity"}
	} else if !slices.Contains(skip, ".celerity") {
		// Always skipped, since it holds the CLI's own build output and
		// watching it makes every rebuild trigger the next one.
		skip = append(slices.Clone(skip), ".celerity")
	}
	return WatchPaths{
		Roots:      roots,
		Extensions: c.WatchExtensions,
		SkipDirs:   skip,
	}
}
