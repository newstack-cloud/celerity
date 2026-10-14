package build

import (
	"fmt"
	"strings"
)

// Registry holds build components keyed by deploy target or runtime prefix.
type Registry struct {
	strategies map[string]Strategy
	installers map[string]DependencyInstaller
	compilers  map[string]SourceCompiler
	depReaders map[string]HandlerDependencyReader
}

func NewRegistry() *Registry {
	return &Registry{
		strategies: map[string]Strategy{},
		installers: map[string]DependencyInstaller{},
		compilers:  map[string]SourceCompiler{},
		depReaders: map[string]HandlerDependencyReader{},
	}
}

func (r *Registry) RegisterStrategy(deployTarget string, s Strategy) {
	r.strategies[deployTarget] = s
}

func (r *Registry) RegisterDependencyInstaller(runtimePrefix string, i DependencyInstaller) {
	r.installers[runtimePrefix] = i
}

func (r *Registry) RegisterSourceCompiler(runtimePrefix string, c SourceCompiler) {
	r.compilers[runtimePrefix] = c
}

func (r *Registry) RegisterHandlerDependencyReader(runtimePrefix string, reader HandlerDependencyReader) {
	r.depReaders[runtimePrefix] = reader
}

func (r *Registry) GetStrategy(deployTarget string) (Strategy, error) {
	s, ok := r.strategies[deployTarget]
	if !ok {
		return nil, fmt.Errorf("no build strategy registered for deploy target %q", deployTarget)
	}
	return s, nil
}

func (r *Registry) GetDependencyInstaller(runtime string) (DependencyInstaller, error) {
	return findByPrefix(r.installers, runtime, "dependency installer")
}

// GetSourceCompiler returns nil if no compiler is registered (source is
// deployed as-is, e.g. Python).
func (r *Registry) GetSourceCompiler(runtime string) SourceCompiler {
	c, _ := longestPrefixMatch(r.compilers, runtime)
	return c
}

func (r *Registry) GetHandlerDependencyReader(runtime string) HandlerDependencyReader {
	reader, _ := longestPrefixMatch(r.depReaders, runtime)
	return reader
}

func findByPrefix[T any](m map[string]T, runtime, label string) (T, error) {
	v, ok := longestPrefixMatch(m, runtime)
	if !ok {
		var zero T
		return zero, fmt.Errorf("no %s registered for runtime %q", label, runtime)
	}
	return v, nil
}

// Returns the value whose key is the longest prefix of runtime. Longest rather
// than first, because Go map iteration is unordered and a first-match loop
// returns a different value from run to run wherever two registered prefixes
// both match. Nothing registered today overlaps, which is what would make such
// a bug so slow to find.
func longestPrefixMatch[T any](m map[string]T, runtime string) (T, bool) {
	var (
		best    T
		found   bool
		longest int
	)
	for prefix, value := range m {
		if !strings.HasPrefix(runtime, prefix) {
			continue
		}
		if !found || len(prefix) > longest {
			best, found, longest = value, true, len(prefix)
		}
	}
	return best, found
}
