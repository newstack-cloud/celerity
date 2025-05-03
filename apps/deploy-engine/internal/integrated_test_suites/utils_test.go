package integratedtestsuites

import (
	"os"
	"path"
)

func testPluginPaths() (string, string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	pluginPath := path.Join(
		workingDir,
		"__testdata",
		"plugins",
		"bin",
	)
	logFileRootDir := path.Join(
		workingDir,
		"__testdata",
		"tmp",
		"logs",
	)

	return pluginPath, logFileRootDir, nil
}

func testBlueprintDirectory() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	testBlueprintDir := path.Join(
		workingDir,
		"__testdata",
	)

	return testBlueprintDir, nil
}
