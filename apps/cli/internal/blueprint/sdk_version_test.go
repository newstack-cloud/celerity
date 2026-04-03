package blueprint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectSDKVersion_Node_PeerDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"peerDependencies": { "@celerity-sdk/core": "^0.3.1" }
	}`)
	v, err := DetectSDKVersion(dir, "nodejs24.x")
	require.NoError(t, err)
	assert.Equal(t, "0.3.1", v)
}

func TestDetectSDKVersion_Node_Dependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": { "@celerity-sdk/core": "~0.4.0" }
	}`)
	v, err := DetectSDKVersion(dir, "nodejs24.x")
	require.NoError(t, err)
	assert.Equal(t, "0.4.0", v)
}

func TestDetectSDKVersion_Node_DevDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"devDependencies": { "@celerity-sdk/core": "0.5.2" }
	}`)
	v, err := DetectSDKVersion(dir, "nodejs24.x")
	require.NoError(t, err)
	assert.Equal(t, "0.5.2", v)
}

func TestDetectSDKVersion_Node_PeerTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"peerDependencies": { "@celerity-sdk/core": "^1.0.0" },
		"devDependencies": { "@celerity-sdk/core": "^1.2.0" }
	}`)
	v, err := DetectSDKVersion(dir, "nodejs24.x")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", v)
}

func TestDetectSDKVersion_Node_MissingPackage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": { "express": "^4.0.0" }
	}`)
	_, err := DetectSDKVersion(dir, "nodejs24.x")
	assert.Error(t, err)
}

func TestDetectSDKVersion_Node_NoFile(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectSDKVersion(dir, "nodejs24.x")
	assert.Error(t, err)
}

func TestDetectSDKVersion_Python_GreaterEqual(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
dependencies = [
    "celerity-sdk>=0.3.1",
]
`)
	v, err := DetectSDKVersion(dir, "python3.13")
	require.NoError(t, err)
	assert.Equal(t, "0.3.1", v)
}

func TestDetectSDKVersion_Python_WithExtras(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
dependencies = [
    "celerity-sdk[cli]>=0.4.0",
]
`)
	v, err := DetectSDKVersion(dir, "python3.13")
	require.NoError(t, err)
	assert.Equal(t, "0.4.0", v)
}

func TestDetectSDKVersion_Python_TildeEqual(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
dependencies = [
    "celerity-sdk~=0.5.0",
]
`)
	v, err := DetectSDKVersion(dir, "python3.13")
	require.NoError(t, err)
	assert.Equal(t, "0.5.0", v)
}

func TestDetectSDKVersion_Python_ExactEqual(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `
[project]
dependencies = [
    "celerity-sdk==1.0.0",
]
`)
	v, err := DetectSDKVersion(dir, "python3.13")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", v)
}

func TestDetectSDKVersion_Python_NoFile(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectSDKVersion(dir, "python3.13")
	assert.Error(t, err)
}

func TestDetectSDKVersion_Go_Unsupported(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectSDKVersion(dir, "go1.22")
	assert.Error(t, err)
}

func TestDetectSDKVersion_UnknownRuntime(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectSDKVersion(dir, "ruby3.3")
	assert.Error(t, err)
}

func TestStripVersionPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^0.3.1", "0.3.1"},
		{"~0.3.1", "0.3.1"},
		{">=0.3.1", "0.3.1"},
		{">0.3.1", "0.3.1"},
		{"==0.3.1", "0.3.1"},
		{"~=0.3.1", "0.3.1"},
		{"0.3.1", "0.3.1"},
		{"v1.0.0", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, stripVersionPrefix(tt.input))
		})
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}
