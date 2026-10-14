package build

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The determinism the build already claimed was only ever tested with a mocked
// installer over unchanged files. A dependency layer is built by re-running the
// package manager, which rewrites every mtime, so the real question is whether
// identical content with different timestamps hashes the same.
func TestArchiveHashIgnoresModificationTimes(t *testing.T) {
	build := func(mtime time.Time) string {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "index.js"), []byte("module.exports = 1\n"), 0644))
		require.NoError(t, os.Chtimes(filepath.Join(src, "index.js"), mtime, mtime))

		art, err := writeArtifact(ArtifactTypeLambdaLayer, src, t.TempDir(), "layer")
		require.NoError(t, err)
		return art.ContentHash
	}

	first := build(time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC))
	second := build(time.Date(2024, 11, 12, 13, 14, 15, 0, time.UTC))

	assert.Equal(t, first, second,
		"a rebuild must hash the same, or the deploy references a layer the change set never created")
}
