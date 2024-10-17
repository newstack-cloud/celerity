package languageservices

import (
	"os"
	"path"
)

func loadTestBlueprintContent(blueprintFileName string) (string, error) {
	bytes, err := os.ReadFile(path.Join("__testdata", blueprintFileName))
	return string(bytes), err
}

const blueprintURI = "file:///blueprint.yaml"
