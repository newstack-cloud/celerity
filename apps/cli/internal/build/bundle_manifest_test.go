package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func esmProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "package.json"),
		[]byte(`{"type": "module"}`),
		0644,
	))
	return dir
}

// Compiling replaces the copied source tree with the compiler's output, which
// carries no package.json, so Node read the emitted .js files as CommonJS and
// every handler failed on "Cannot use import statement outside a module".
func TestWriteBundleModuleManifest_declaresESMForACompiledBundle(t *testing.T) {
	staging := t.TempDir()

	require.NoError(t, writeBundleModuleManifest("nodejs22.x", esmProjectDir(t), staging))

	written, err := os.ReadFile(filepath.Join(staging, "package.json"))
	require.NoError(t, err)
	require.Contains(t, string(written), `"type": "module"`)
}

// A CommonJS project needs no declaration; writing one would change how its own
// files are read.
func TestWriteBundleModuleManifest_leavesCommonJSAlone(t *testing.T) {
	appDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(appDir, "package.json"), []byte(`{"type": "commonjs"}`), 0644))
	staging := t.TempDir()

	require.NoError(t, writeBundleModuleManifest("nodejs22.x", appDir, staging))

	_, err := os.Stat(filepath.Join(staging, "package.json"))
	require.True(t, os.IsNotExist(err))
}

// A manifest already in the bundle came from the project's own source tree,
// where the author decides what it says.
func TestWriteBundleModuleManifest_doesNotOverwriteTheProjectManifest(t *testing.T) {
	staging := t.TempDir()
	authored := `{"type": "module", "imports": {"#app/*": "./*"}}`
	require.NoError(t, os.WriteFile(
		filepath.Join(staging, "package.json"), []byte(authored), 0644))

	require.NoError(t, writeBundleModuleManifest("nodejs22.x", esmProjectDir(t), staging))

	written, err := os.ReadFile(filepath.Join(staging, "package.json"))
	require.NoError(t, err)
	require.Equal(t, authored, string(written))
}

func TestWriteBundleModuleManifest_leavesNonNodeRuntimesAlone(t *testing.T) {
	staging := t.TempDir()

	require.NoError(t, writeBundleModuleManifest("python3.13", esmProjectDir(t), staging))

	_, err := os.Stat(filepath.Join(staging, "package.json"))
	require.True(t, os.IsNotExist(err))
}
