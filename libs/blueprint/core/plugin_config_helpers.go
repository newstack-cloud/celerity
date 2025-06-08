package core

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// PluginConfig is a convenience type that wraps a map of string keys
// to scalar values holding the configuration for a provider or transformer plugin.
// This enhances a map to allow for convenience methods such as retrieving
// all config values under a specific prefix or deriving a slice or map
// from a key prefix.
type PluginConfig map[string]*ScalarValue

// Get retrieves a configuration value by its key.
// It returns the value and a boolean indicating whether the key exists in the config.
func (c PluginConfig) Get(key string) (*ScalarValue, bool) {
	value, ok := c[key]
	return value, ok
}

// GetAllWithPrefix returns a subset of the PluginConfig
// that contains all keys that start with the specified prefix.
func (c PluginConfig) GetAllWithPrefix(prefix string) PluginConfig {
	if prefix == "" {
		return c
	}

	configValues := map[string]*ScalarValue{}

	for key, value := range c {
		if strings.HasPrefix(key, prefix) {
			configValues[key] = value
		}
	}

	return configValues
}

type keyWithPosition struct {
	Key string
	Pos int
}

// GetAllWithSlicePrefix returns a subset of the PluginConfig
// that contains all keys that match the pattern "<prefix>.<integer>".
// This is useful for retrieving configuration values that represent
// a list of objects or lists.
// For example, if the prefix is "aws.config.regionKMSKeys",
// it will return all values with a key that match the pattern
// "^aws\.config\.regionKMSKeys\.[0-9]+\.?.*".
// The second return value is a slice of the keys in order based on
// the matching index.
func (c PluginConfig) GetAllWithSlicePrefix(prefix string) (PluginConfig, []string) {
	if prefix == "" {
		return c, nil
	}

	configValues := map[string]*ScalarValue{}
	keysWithPositions := []*keyWithPosition{}

	escapedPrefix := escapeRegexpSpecialChars(prefix)
	pattern, err := regexp.Compile(`^` + escapedPrefix + `\.([0-9]+)\.?`)
	if err != nil {
		return configValues, nil
	}

	for key, value := range c {
		matches := pattern.FindStringSubmatch(key)
		if len(matches) > 1 {
			pos, err := strconv.Atoi(matches[1])
			if err == nil {
				keysWithPositions = append(keysWithPositions, &keyWithPosition{
					Key: key,
					Pos: pos,
				})
				configValues[key] = value
			}
		}
	}

	slices.SortFunc(keysWithPositions, func(a, b *keyWithPosition) int {
		if a.Pos < b.Pos {
			return -1
		}

		if a.Pos > b.Pos {
			return 1
		}

		return 0
	})

	keys := make([]string, 0, len(keysWithPositions))
	for _, keyWithPos := range keysWithPositions {
		keys = append(keys, keyWithPos.Key)
	}

	return configValues, keys
}

// GetAllWithMapPrefix returns a subset of the PluginConfig
// that contains all keys that match the pattern "<prefix>.<string>".
// This is useful for retrieving configuration values that represent
// a map of objects or lists.
func (c PluginConfig) GetAllWithMapPrefix(prefix string) PluginConfig {
	if prefix == "" {
		return c
	}
	configValues := map[string]*ScalarValue{}

	escapedPrefix := escapeRegexpSpecialChars(prefix)
	pattern, err := regexp.Compile(`^` + escapedPrefix + `\.([A-Za-z0-9\-_]+)$`)
	if err != nil {
		return configValues
	}

	for key, value := range c {
		matches := pattern.FindStringSubmatch(key)
		if len(matches) > 1 {
			configValues[key] = value
		}
	}

	return configValues
}

type valueWithPosition struct {
	Pos   int
	Value *ScalarValue
}

// SliceFromPrefix returns a slice of all configuration values
// that start with the specified prefix where the prefix is followed by ".<integer>".
// For example, if the prefix is "aws.config.regionKMSKeys",
// it will return all values with a key that match the pattern "^aws\.config\.regionKMSKeys\.[0-9]+$".
// This only works for keys that represent a list of scalar values,
// not for nested maps or lists, for nested structures, you can use
// GetAllWithSlicePrefix to retrieve a subset of config values with a specific array prefix.
func (c PluginConfig) SliceFromPrefix(prefix string) []*ScalarValue {
	valuesWithPositions := []*valueWithPosition{}

	if prefix == "" {
		return []*ScalarValue{}
	}

	escapedPrefix := escapeRegexpSpecialChars(prefix)
	pattern, err := regexp.Compile(`^` + escapedPrefix + `\.([0-9]+)$`)
	if err != nil {
		return []*ScalarValue{}
	}

	for key, value := range c {
		matches := pattern.FindStringSubmatch(key)
		if len(matches) > 1 {
			pos, err := strconv.Atoi(matches[1])
			if err == nil {
				valuesWithPositions = append(valuesWithPositions, &valueWithPosition{
					Pos:   pos,
					Value: value,
				})
			}
		}
	}

	slices.SortFunc(valuesWithPositions, func(a, b *valueWithPosition) int {
		if a.Pos < b.Pos {
			return -1
		}

		if a.Pos > b.Pos {
			return 1
		}

		return 0
	})

	configValues := make([]*ScalarValue, len(valuesWithPositions))
	for i, valueWithPos := range valuesWithPositions {
		configValues[i] = valueWithPos.Value
	}

	return configValues
}

// MapFromPrefix returns a map of all configuration values
// that start with the specified prefix where the prefix is followed by ".<string>".
// For example, if the prefix is "aws.config.regionKMSKeys",
// it will return all values with a key that match the pattern "^aws\.config\.regionKMSKeys\.[A-Za-z0-9\-_]+$".
func (c PluginConfig) MapFromPrefix(prefix string) PluginConfig {
	values := PluginConfig{}

	if prefix == "" {
		return values
	}

	escapedPrefix := escapeRegexpSpecialChars(prefix)
	pattern, err := regexp.Compile(`^` + escapedPrefix + `\.([A-Za-z0-9\-_]+)$`)
	if err != nil {
		return values
	}

	for key, value := range c {
		matches := pattern.FindStringSubmatch(key)
		if len(matches) > 1 {
			// The final map should contain the key without the config prefix,
			// this is consistent with the behaviour of SliceFromPrefix that returns
			// a slice of values sorted by the matching index.
			values[matches[1]] = value
		}
	}

	return values
}
