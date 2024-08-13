package schema

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

// Loader provides a function that loads a serialised blueprint
// spec either from a file path or a serialised string already
// loaded in memory.
type Loader func(string, SpecFormat) (*Blueprint, error)

// Load deals with loading a blueprint specification
// from a YAML or JSON file on disk.
func Load(specFilePath string, inputFormat SpecFormat) (*Blueprint, error) {
	blueprint := &Blueprint{}
	f, err := os.Open(specFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if inputFormat == YAMLSpecFormat {
		err = yaml.NewDecoder(f).Decode(blueprint)
	} else {
		err = json.NewDecoder(f).Decode(blueprint)
	}

	return blueprint, err
}

// SpecFormat is the format of a specification
// to be used for transport and storage.
type SpecFormat string

const (
	// JSONSpecFormat determines that a spec being loaded
	// or exported should be serialised or deserialised
	// in the JSON format.
	JSONSpecFormat SpecFormat = "json"
	// YAMLSpecFormat determines that a spec being loaded
	// or exported should be serialised or deserialised
	// in the YAML format.
	YAMLSpecFormat SpecFormat = "yaml"
)

// LoadString deals with loading a blueprint specification
// from a given YAML or JSON string.
func LoadString(spec string, inputFormat SpecFormat) (*Blueprint, error) {
	blueprint := &Blueprint{}

	var err error
	if inputFormat == YAMLSpecFormat {
		err = yaml.Unmarshal([]byte(spec), blueprint)
	} else {
		err = json.Unmarshal([]byte(spec), blueprint)
	}

	return blueprint, err
}
