package substitutions

const (
	// SubstitutionFunctionFromJSON is a function that is used to extract values
	// from a serialised JSON string.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the json string to extract values from.
	// 	2. string - A valid json pointer expression to extract the value from
	//             the JSON string.
	//
	// Returns:
	// 	The value extracted from the JSON string. This could be a primitive value,
	// 	an array or a mapping.
	//
	// Examples:
	//  With a reference:
	// 	${fromjson(variables.cacheClusterConfig, "host")}
	//
	//  With a function call:
	// 	${fromjson(trim(variables.cacheClusterConfig), "host")}
	//
	//  With a string literal:
	// 	${fromjson("{\"host\":\"localhost\"}", "host")}
	//
	SubstitutionFunctionFromJSON SubstitutionFunctionName = "fromjson"
	// SubstitutionFunctionJSONDecode is a function that is used to decode a serialised json string
	// into an array or mapping.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the json string to decode.
	//
	// Returns:
	// 	The value decoded from the JSON string. This could be an array or a mapping.
	//
	// Example:
	//  ${jsondecode((variables.cacheClusterConfig))}
	SubstitutionFunctionJSONDecode SubstitutionFunctionName = "jsondecode"
	// SubstitutionFunctionLen is a function that is used to get the length of a string, array
	// or mapping.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the value to get the length of.
	//
	// Returns:
	// 	The length of the value. This could be the length of a string, array or mapping.
	//
	// Example:
	// 	${len(variables.cacheClusterConfig.endpoints)}
	SubstitutionFunctionLen SubstitutionFunctionName = "len"
	// SubstitutionFunctionSubstr is a function that is used to get a substring of a given string.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the value to get the substring from.
	// 	2. integer - The index of the first character to include in the substring.
	// 	3. integer (optional) - The index of the last character to include in the substring. If not
	//					        provided, the substring will include all characters from the start index
	//							to the end of the string.
	//
	// Returns:
	// 	The substring from the given string.
	//
	// Example:
	// 	${substr(variables.cacheClusterConfig.endpoints[0].host, 0, 3)}
	SubstitutionFunctionSubstr SubstitutionFunctionName = "substr"
	// Replace is a function that is used to replace all occurrences of a given string with another string.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the value to replace.
	// 	2. string - The "search" substring to replace.
	// 	3. string - The substring to replace the  "search" substring with.
	//
	// Returns:
	// 	The input string with all occurrences of the "search" substring replaced with the "replace" substring.
	//
	// Example:
	// 	${replace(variables.cacheClusterConfig.host, "http://", "https://")}
	SubstitutionFunctionReplace SubstitutionFunctionName = "replace"
	// SubstitutionFunctionTrim is a function that is used to remove all leading and trailing whitespace
	// from a given string.
	//
	// Parameters:
	// 	1. string - A valid string literal, reference or function call yielding the value to trim.
	//
	// Returns:
	// 	The input string with all leading and trailing whitespace removed.
	//
	// Example:
	// 	${trim(variables.cacheClusterConfig.host)}
	SubstitutionFunctionTrim SubstitutionFunctionName = "trim"
	// SubstitutionFunctionTrimPrefix is a function that is used to remove a prefix from a string.
	//
	// Parameters:
	//	1. string - A valid string literal, reference or function call yielding a return value representing
	// 				the string to remove the prefix from.
	//  2. string - The prefix to remove from the string.
	//
	// Returns:
	// 	The input string with the prefix removed.
	//
	// Example:
	// 	${trimprefix(variables.cacheClusterConfig.host, "http://")}
	SubstitutionFunctionTrimPrefix SubstitutionFunctionName = "trimprefix"
	// SubstitutionFunctionTrimSuffix is a function that is used to remove a suffix from a string.
	//
	// Parameters:
	//	1. string - A valid string literal, reference or function call yielding a return value representing
	// 				the string to remove the suffix from.
	//  2. string - The suffix to remove from the string.
	//
	// Returns:
	// 	The input string with the suffix removed.
	//
	// Example:
	// 	${trimsuffix(variables.cacheClusterConfig.host, ":3000")}
	SubstitutionFunctionTrimSuffix SubstitutionFunctionName = "trimsuffix"
)

var (
	// SubstitutionFunctions provides a slice of all the supported
	// functions that can be called in a substitution within ${..}.
	SubstitutionFunctions = []SubstitutionFunctionName{
		SubstitutionFunctionFromJSON,
		SubstitutionFunctionJSONDecode,
		SubstitutionFunctionLen,
		SubstitutionFunctionSubstr,
		SubstitutionFunctionReplace,
		SubstitutionFunctionTrim,
		SubstitutionFunctionTrimPrefix,
		SubstitutionFunctionTrimSuffix,
	}
)
