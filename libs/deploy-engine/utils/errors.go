package utils

import "github.com/two-hundred/celerity/libs/deploy-engine/errors"

func errUnsupportedBlueprintFormat(fileName string) error {
	return &errors.DeployEngineError{
		Message: "unsupported blueprint format file \"" + fileName +
			"\", only json or yaml files with extensions are supported",
	}
}
