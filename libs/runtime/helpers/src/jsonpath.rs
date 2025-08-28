use jsonpath_rust::JsonPath;
use serde_json::Value;

/// Injects a value into the root JSON object at the specified JSON path.
/// If the path is valid for the provided JSON object,
/// the value is injected and the function returns true.
/// If the path already exists in the JSON object, the value is replaced.
/// If the path is not valid for the provided JSON object,
/// the value is not injected and the function returns false.
///
/// This function **only** supports injecting fields into to the root JSON object.
///
/// An example of how you might use this function:
/// ```
/// # use serde_json::json;
///
/// let mut json = json!({
///    "name": "John Doe",
/// });
///
/// let path = JsonPath::from_str("$.planInfo").unwrap();
/// let value = json!({
///    "planId": "1",
///    "planName": "premium",
///    "planType": "annual",
///    "planPrice": 99.99,
/// });
/// let result = jsonpath_inject_root(&path, &mut json, value);
/// assert_eq!(result, false);
/// assert_eq!(json, json!({
///   "name": "John Doe",
///   "planInfo": {
///     "planId": "1",
///     "planName": "premium",
///     "planType": "annual",
///     "planPrice": 99.99,
///   },
/// }));
/// ```
pub fn jsonpath_inject_root(path: &JsonPath, inject_into: &mut Value, inject_value: Value) -> bool {
    match inject_into {
        Value::Object(ref mut map) => {
            let field = jsonpath_get_root_object_field(path);
            if let Some(field) = field {
                map.insert(field, inject_value);
                true
            } else {
                false
            }
        }
        _ => false,
    }
}

fn jsonpath_get_root_object_field(path: &JsonPath) -> Option<String> {
    match path {
        JsonPath::Chain(chain) => {
            if chain.is_empty() || chain.len() > 2 {
                return None;
            }
            let mut field = None;
            let mut i = 0;
            while field.is_none() && i < chain.len() {
                match chain[i] {
                    JsonPath::Root => {
                        // Root is the first element in the chain, so we can skip it.
                    }
                    JsonPath::Field(ref key) => {
                        field = Some(key);
                    }
                    _ => {
                        return None;
                    }
                }
                i += 1;
            }
            field.cloned()
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use std::str::FromStr;

    use super::*;
    use serde_json::json;

    #[test]
    fn test_jsonpath_inject_root_dot_notation() {
        let mut json = json!({
            "name": "John Doe",
        });

        let path = JsonPath::from_str("$.planInfo").unwrap();
        let value = json!({
            "planId": "1",
            "planName": "premium",
            "planType": "annual",
            "planPrice": 99.99,
        });
        let result = jsonpath_inject_root(&path, &mut json, value);
        assert!(result);
        assert_eq!(
            json,
            json!({
                "name": "John Doe",
                "planInfo": {
                    "planId": "1",
                    "planName": "premium",
                    "planType": "annual",
                    "planPrice": 99.99,
                },
            })
        );
    }

    #[test]
    fn test_jsonpath_inject_root_bracket_notation() {
        let mut json = json!({
            "name": "John Doe",
        });

        let path = JsonPath::from_str("$['planInfo']").unwrap();
        let value = json!({
            "planId": "1",
            "planName": "premium",
            "planType": "annual",
            "planPrice": 99.99,
        });
        let result = jsonpath_inject_root(&path, &mut json, value);
        assert!(result);
        assert_eq!(
            json,
            json!({
                "name": "John Doe",
                "planInfo": {
                    "planId": "1",
                    "planName": "premium",
                    "planType": "annual",
                    "planPrice": 99.99,
                },
            })
        );
    }

    #[test]
    fn test_fails_to_inject_nested_path() {
        let mut json = json!({
            "name": "John Doe",
        });

        let path = JsonPath::from_str("$.planInfo.planId").unwrap();
        let value = json!({
            "planId": "1",
            "planName": "premium",
            "planType": "annual",
            "planPrice": 99.99,
        });
        let result = jsonpath_inject_root(&path, &mut json, value);
        assert!(!result);
        assert_eq!(
            json,
            json!({
                "name": "John Doe",
            })
        );
    }

    #[test]
    fn test_fails_to_inject_array_path() {
        let mut json = json!({
            "name": "John Doe",
        });

        let path = JsonPath::from_str("$[0]").unwrap();
        let value = json!({
            "planId": "1",
            "planName": "premium",
            "planType": "annual",
            "planPrice": 99.99,
        });
        let result = jsonpath_inject_root(&path, &mut json, value);
        assert!(!result);
        assert_eq!(
            json,
            json!({
                "name": "John Doe",
            })
        );
    }
}
