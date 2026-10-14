package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Lambda mounts a layer at /opt and Node resolves packages from
// /opt/nodejs/node_modules. npm creates node_modules inside whatever directory
// it installs into, so the install target must stop at "nodejs". Naming
// node_modules here put every package one level too deep, nothing in the layer
// resolved, and every handler failed during init on a module resolution error.
func TestLambdaLayerSubdir_nodeStopsAboveNodeModules(t *testing.T) {
	assert.Equal(t, "nodejs", lambdaLayerSubdir("nodejs22.x"))
	assert.Equal(t, "nodejs", lambdaLayerSubdir("nodejs24.x"))
}

// pip installs into the directory it is given rather than creating one, so the
// Python path is the full site-packages location.
func TestLambdaLayerSubdir_pythonKeepsTheFullSitePackagesPath(t *testing.T) {
	assert.Equal(t, "python/lib/python3.13/site-packages", lambdaLayerSubdir("python3.13"))
}
