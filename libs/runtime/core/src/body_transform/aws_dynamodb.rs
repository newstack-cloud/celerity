//! AWS DynamoDB Streams body transformer.
//!
//! Transforms DynamoDB stream record format into the Celerity-standard
//! datastore event shape consumed by SDK handlers.
//!
//! # Input format (DynamoDB Streams)
//!
//! ```json
//! {
//!   "Keys": { "id": { "S": "123" } },
//!   "NewImage": { "id": { "S": "123" }, "name": { "S": "Alice" } },
//!   "OldImage": { "id": { "S": "123" }, "name": { "S": "Bob" } }
//! }
//! ```
//!
//! # Output format (Celerity-standard)
//!
//! ```json
//! {
//!   "keys": { "id": "123" },
//!   "newItem": { "id": "123", "name": "Alice" },
//!   "oldItem": { "id": "123", "name": "Bob" }
//! }
//! ```

use serde_json::Value;

/// Transforms a DynamoDB stream record body into `{ keys?, newItem?, oldItem? }`.
///
/// Unmarshalls DynamoDB attribute format (`{"S": "value"}`) to plain JSON.
/// Returns the original body unchanged on parse failure or when no recognised
/// DynamoDB fields are present.
pub fn transform(body: &str) -> String {
    let parsed: Value = match serde_json::from_str(body) {
        Ok(v) => v,
        Err(_) => return body.to_string(),
    };

    let keys = parsed.get("Keys").map(unmarshall_item);
    let new_image = parsed.get("NewImage").map(unmarshall_item);
    let old_image = parsed.get("OldImage").map(unmarshall_item);

    if keys.is_none() && new_image.is_none() && old_image.is_none() {
        return body.to_string();
    }

    let mut result = serde_json::Map::new();
    if let Some(k) = keys {
        result.insert("keys".to_string(), k);
    }
    if let Some(ni) = new_image {
        result.insert("newItem".to_string(), ni);
    }
    if let Some(oi) = old_image {
        result.insert("oldItem".to_string(), oi);
    }

    serde_json::to_string(&Value::Object(result)).unwrap_or_else(|_| body.to_string())
}

/// Maps a DynamoDB Streams event name to a Celerity-standard datastore event
/// type.
pub fn map_event_type(event_name: &str) -> Option<&'static str> {
    match event_name {
        "INSERT" => Some("inserted"),
        "MODIFY" => Some("modified"),
        "REMOVE" => Some("removed"),
        _ => None,
    }
}

// ---------------------------------------------------------------------------
// DynamoDB attribute unmarshalling
// ---------------------------------------------------------------------------

/// Converts a single DynamoDB attribute value to a plain JSON value.
///
/// Supports all DynamoDB attribute types: `S`, `N`, `BOOL`, `NULL`, `L`, `M`,
/// `SS`, `NS`, `BS`.
fn unmarshall_attribute(attr: &Value) -> Value {
    let Some(obj) = attr.as_object() else {
        return attr.clone();
    };

    if let Some(s) = obj.get("S") {
        return s.clone();
    }
    if let Some(n) = obj.get("N") {
        return parse_dynamo_number(n);
    }
    if let Some(b) = obj.get("BOOL") {
        return b.clone();
    }
    if obj.contains_key("NULL") {
        return Value::Null;
    }
    if let Some(l) = obj.get("L").and_then(Value::as_array) {
        return Value::Array(l.iter().map(unmarshall_attribute).collect());
    }
    if let Some(m) = obj.get("M").and_then(Value::as_object) {
        return unmarshall_map(m);
    }
    if let Some(ss) = obj.get("SS") {
        return ss.clone();
    }
    if let Some(ns) = obj.get("NS").and_then(Value::as_array) {
        return Value::Array(ns.iter().map(parse_dynamo_number).collect());
    }
    if let Some(bs) = obj.get("BS") {
        return bs.clone();
    }

    attr.clone()
}

/// Parses a DynamoDB `N` (number) value, which is always a JSON string.
///
/// Tries integer first, then float, falling back to the original value.
fn parse_dynamo_number(n: &Value) -> Value {
    let Some(n_str) = n.as_str() else {
        return n.clone();
    };

    if let Ok(i) = n_str.parse::<i64>() {
        return Value::Number(i.into());
    }
    if let Ok(f) = n_str.parse::<f64>() {
        if let Some(num) = serde_json::Number::from_f64(f) {
            return Value::Number(num);
        }
    }

    Value::String(n_str.to_string())
}

/// Unmarshalls a DynamoDB attribute map to a plain JSON object.
fn unmarshall_map(map: &serde_json::Map<String, Value>) -> Value {
    Value::Object(
        map.iter()
            .map(|(k, v)| (k.clone(), unmarshall_attribute(v)))
            .collect(),
    )
}

/// Unmarshalls a DynamoDB item (attribute map) to a plain JSON object.
fn unmarshall_item(item: &Value) -> Value {
    match item.as_object() {
        Some(map) => unmarshall_map(map),
        None => item.clone(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    // -----------------------------------------------------------------------
    // Body transform
    // -----------------------------------------------------------------------

    #[test]
    fn unmarshalls_full_record() {
        let body = concat!(
            r#"{"Keys":{"id":{"S":"123"}},"#,
            r#""NewImage":{"id":{"S":"123"},"name":{"S":"Alice"},"score":{"N":"99"}}}"#,
        );
        let result = transform(body);
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["keys"]["id"], "123");
        assert_eq!(parsed["newItem"]["name"], "Alice");
        assert_eq!(parsed["newItem"]["score"], 99);
    }

    #[test]
    fn returns_original_on_invalid_json() {
        assert_eq!(transform("not json"), "not json");
    }

    #[test]
    fn returns_original_when_no_dynamo_fields() {
        let body = r#"{"someOtherField": true}"#;
        assert_eq!(transform(body), body);
    }

    #[test]
    fn keys_only() {
        let body = r#"{"Keys":{"pk":{"S":"abc"}}}"#;
        let result = transform(body);
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["keys"]["pk"], "abc");
        assert!(parsed.get("newItem").is_none());
        assert!(parsed.get("oldItem").is_none());
    }

    #[test]
    fn modify_with_old_and_new_image() {
        let body = concat!(
            r#"{"Keys":{"userId":{"S":"u1"}},"#,
            r#""OldImage":{"userId":{"S":"u1"},"age":{"N":"29"}},"#,
            r#""NewImage":{"userId":{"S":"u1"},"age":{"N":"30"}}}"#,
        );
        let result = transform(body);
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["keys"]["userId"], "u1");
        assert_eq!(parsed["oldItem"]["age"], 29);
        assert_eq!(parsed["newItem"]["age"], 30);
    }

    // -----------------------------------------------------------------------
    // Event type mapping
    // -----------------------------------------------------------------------

    #[test]
    fn event_type_mapping() {
        assert_eq!(map_event_type("INSERT"), Some("inserted"));
        assert_eq!(map_event_type("MODIFY"), Some("modified"));
        assert_eq!(map_event_type("REMOVE"), Some("removed"));
        assert_eq!(map_event_type("UPDATE"), None);
        assert_eq!(map_event_type("DELETE"), None);
        assert_eq!(map_event_type(""), None);
    }

    // -----------------------------------------------------------------------
    // Attribute unmarshalling
    // -----------------------------------------------------------------------

    #[test]
    fn unmarshall_string() {
        assert_eq!(unmarshall_attribute(&json!({"S": "hello"})), json!("hello"));
    }

    #[test]
    fn unmarshall_number_int_and_float() {
        assert_eq!(unmarshall_attribute(&json!({"N": "42"})), json!(42));
        assert_eq!(unmarshall_attribute(&json!({"N": "3.14"})), json!(3.14));
    }

    #[test]
    fn unmarshall_bool_and_null() {
        assert_eq!(unmarshall_attribute(&json!({"BOOL": true})), json!(true));
        assert_eq!(unmarshall_attribute(&json!({"NULL": true})), Value::Null);
    }

    #[test]
    fn unmarshall_list() {
        let attr = json!({"L": [{"S": "a"}, {"N": "1"}]});
        assert_eq!(unmarshall_attribute(&attr), json!(["a", 1]));
    }

    #[test]
    fn unmarshall_nested_map() {
        let attr = json!({"M": {"name": {"S": "John"}, "age": {"N": "30"}}});
        assert_eq!(
            unmarshall_attribute(&attr),
            json!({"name": "John", "age": 30})
        );
    }

    #[test]
    fn unmarshall_string_set() {
        let attr = json!({"SS": ["a", "b", "c"]});
        assert_eq!(unmarshall_attribute(&attr), json!(["a", "b", "c"]));
    }

    #[test]
    fn unmarshall_number_set() {
        let attr = json!({"NS": ["1", "2", "3"]});
        assert_eq!(unmarshall_attribute(&attr), json!([1, 2, 3]));
    }
}
