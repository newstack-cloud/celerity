package utils

import "github.com/two-hundred/celerity/libs/build-engine/errors"

func errUnsupportedBlueprintFormat(fileName string) error {
	return &errors.BuildEngineError{
		Message: "unsupported blueprint format file \"" + fileName +
			"\", only json or yaml files with extensions are supported",
	}
}
