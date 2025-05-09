package testhelpers

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bradleyjkemp/cupaloy/v2"
)

// Snapshot is a wrapper function around cupaloy/v2
// to automatically generate snapshot names that can
// safely be used as file names on all platforms.
func Snapshot(values ...any) error {
	return cupaloy.SnapshotWithName(
		getNameOfCaller(),
		values...,
	)
}

func getNameOfCaller() string {
	// first caller is the caller of this function, we want the caller of our caller
	pc, _, _, _ := runtime.Caller(2)
	fullPath := runtime.FuncForPC(pc).Name()
	packageFunctionName := filepath.Base(fullPath)

	dotsReplaced := strings.Replace(packageFunctionName, ".", "-", -1)
	// Remove "*" from pointer receivers since they can not be used in file names
	// on windows systems.
	return strings.Replace(dotsReplaced, "*", "", -1)
}
