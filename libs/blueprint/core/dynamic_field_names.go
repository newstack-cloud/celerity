package core

import "strings"

// IsDynamicFieldName checks if a given field definition name
// contains one or more "<placeholder>" sub-strings.
// This is intended to be used for plugin config field definitions
// and link annotation definitions.
func IsDynamicFieldName(fieldDefName string) bool {
	indexOpenAngleBracket := strings.Index(fieldDefName, "<")
	indexCloseAngleBracket := strings.Index(fieldDefName, ">")
	return indexOpenAngleBracket != -1 &&
		indexCloseAngleBracket != -1 &&
		indexOpenAngleBracket < indexCloseAngleBracket
}

const (
	capturePlaceholderPatternString = "([A-Za-z0-9\\-_]+)"
)

// CreatePatternForDynamicFieldName creates a regex pattern
// for a given field definition name that contains one or more
// "<placeholder>" sub-strings.
// This is intended to be used for plugin config field definitions
// and link annotation definitions.
// Set maxPlaceholders to a value of -1 to allow for any number of
// placeholders to be captured.
func CreatePatternForDynamicFieldName(fieldDefName string, maxPlaceholders int) string {
	finalFieldDefName := escapeRegexpSpecialChars(fieldDefName)
	startIndex := 0
	placeholderCount := 0
	for startIndex != -1 &&
		(maxPlaceholders == -1 || placeholderCount < maxPlaceholders) {
		searchIn := finalFieldDefName[startIndex:]
		openIndex := strings.Index(searchIn, "<")
		closeIndex := strings.Index(searchIn, ">")
		if openIndex == -1 || closeIndex == -1 {
			startIndex = -1
		} else {
			finalFieldDefName = finalFieldDefName[:startIndex+openIndex] +
				capturePlaceholderPatternString +
				finalFieldDefName[startIndex+closeIndex+1:]
			startIndex += closeIndex + 1
		}
		placeholderCount += 1
	}
	return finalFieldDefName
}
