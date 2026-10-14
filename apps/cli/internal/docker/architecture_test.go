package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Docker reports the kernel's spelling, which is not Go's.
func TestNormaliseArchitecture(t *testing.T) {
	for reported, want := range map[string]string{
		"aarch64":  "arm64",
		"arm64":    "arm64",
		"arm64/v8": "arm64",
		"x86_64":   "amd64",
		"amd64":    "amd64",
	} {
		assert.Equal(t, want, NormaliseArchitecture(reported), reported)
	}
}

// An architecture with no translation is still the right answer to hand a
// compiler.
func TestNormaliseArchitecture_passesThroughTheUnknown(t *testing.T) {
	assert.Equal(t, "riscv64", NormaliseArchitecture("riscv64"))
}
