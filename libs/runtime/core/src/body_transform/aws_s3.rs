//! AWS S3 bucket event body transformer.
//!
//! Transforms the standard AWS S3 event notification format into the
//! Celerity-standard bucket event shape consumed by SDK handlers.
//!
//! # Input format (AWS S3 / MinIO bridge)
//!
//! ```json
//! {
//!   "Records": [{
//!     "s3": {
//!       "bucket": { "name": "uploads" },
//!       "object": { "key": "photo.jpg", "size": 2048, "eTag": "abc123" }
//!     }
//!   }]
//! }
//! ```
//!
//! # Output format (Celerity-standard)
//!
//! ```json
//! { "key": "photo.jpg", "size": 2048, "eTag": "abc123" }
//! ```

use serde_json::Value;

/// Transforms an S3 notification body into `{ key, size?, eTag? }`.
///
/// Expects the standard AWS notification envelope with a `Records` array.
/// Returns the original body unchanged on parse failure or missing fields.
pub fn transform(body: &str) -> String {
    let parsed: Value = match serde_json::from_str(body) {
        Ok(v) => v,
        Err(_) => return body.to_string(),
    };

    let Some(s3) = extract_s3_object(&parsed) else {
        return body.to_string();
    };

    build_result(s3).unwrap_or_else(|| body.to_string())
}

/// Maps an S3 / MinIO event name to a Celerity-standard bucket event type.
///
/// Handles both the subscription format (`s3:ObjectCreated:Put`) and the
/// notification format (`ObjectCreated:Put`).
pub fn map_event_type(event_name: &str) -> Option<&'static str> {
    let name = event_name.strip_prefix("s3:").unwrap_or(event_name);

    if name.starts_with("ObjectCreated:") || name.starts_with("ObjectRestore:") {
        return Some("created");
    }
    if name.starts_with("ObjectRemoved:") {
        return Some("deleted");
    }
    if name.starts_with("ObjectTagging:") || name.starts_with("ObjectAcl:") {
        return Some("metadataUpdated");
    }

    None
}

/// Navigates into `Records[0].s3` to find the S3 object metadata.
fn extract_s3_object(parsed: &Value) -> Option<&Value> {
    parsed
        .get("Records")
        .and_then(Value::as_array)
        .and_then(|arr| arr.first())
        .and_then(|record| record.get("s3"))
}

/// Builds the Celerity-standard `{ key, size?, eTag? }` result from the S3
/// object metadata.
fn build_result(s3: &Value) -> Option<String> {
    let object = s3.get("object")?;
    let key = object
        .get("key")
        .and_then(Value::as_str)
        .unwrap_or_default();

    let mut result = serde_json::Map::new();
    result.insert("key".to_string(), Value::String(key.to_string()));

    if let Some(size) = object.get("size").and_then(Value::as_u64) {
        result.insert("size".to_string(), Value::Number(size.into()));
    }

    if let Some(etag) = object.get("eTag").and_then(Value::as_str) {
        result.insert("eTag".to_string(), Value::String(etag.to_string()));
    }

    serde_json::to_string(&Value::Object(result)).ok()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn extracts_all_object_fields() {
        let body = r#"{"Records":[{"s3":{"bucket":{"name":"uploads"},"object":{"key":"photo.jpg","size":2048,"eTag":"def456"}}}]}"#;
        let result = transform(body);
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["key"], "photo.jpg");
        assert_eq!(parsed["size"], 2048);
        assert_eq!(parsed["eTag"], "def456");
    }

    #[test]
    fn returns_original_on_invalid_json() {
        assert_eq!(transform("not json"), "not json");
    }

    #[test]
    fn returns_original_when_records_missing() {
        let body = r#"{"noRecords": true}"#;
        assert_eq!(transform(body), body);
    }

    #[test]
    fn handles_delete_event_without_optional_fields() {
        let body =
            r#"{"Records":[{"s3":{"bucket":{"name":"uploads"},"object":{"key":"photo.jpg"}}}]}"#;
        let result = transform(body);
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["key"], "photo.jpg");
        assert!(parsed.get("size").is_none());
        assert!(parsed.get("eTag").is_none());
    }

    #[test]
    fn created_event_variants() {
        assert_eq!(map_event_type("s3:ObjectCreated:Put"), Some("created"));
        assert_eq!(map_event_type("s3:ObjectCreated:Post"), Some("created"));
        assert_eq!(map_event_type("s3:ObjectCreated:Copy"), Some("created"));
        assert_eq!(
            map_event_type("s3:ObjectCreated:CompleteMultipartUpload"),
            Some("created")
        );
        assert_eq!(map_event_type("s3:ObjectRestore:Post"), Some("created"));
        assert_eq!(
            map_event_type("s3:ObjectRestore:Completed"),
            Some("created")
        );
        assert_eq!(map_event_type("ObjectCreated:Put"), Some("created"));
        assert_eq!(map_event_type("ObjectRestore:Completed"), Some("created"));
    }

    #[test]
    fn deleted_event_variants() {
        assert_eq!(map_event_type("s3:ObjectRemoved:Delete"), Some("deleted"));
        assert_eq!(
            map_event_type("s3:ObjectRemoved:DeleteMarkerCreated"),
            Some("deleted")
        );
        assert_eq!(map_event_type("ObjectRemoved:Delete"), Some("deleted"));
    }

    #[test]
    fn metadata_updated_event_variants() {
        assert_eq!(
            map_event_type("s3:ObjectTagging:Put"),
            Some("metadataUpdated")
        );
        assert_eq!(
            map_event_type("s3:ObjectTagging:Delete"),
            Some("metadataUpdated")
        );
        assert_eq!(map_event_type("s3:ObjectAcl:Put"), Some("metadataUpdated"));
        assert_eq!(map_event_type("ObjectTagging:Put"), Some("metadataUpdated"));
    }

    #[test]
    fn unrecognised_events_return_none() {
        assert_eq!(map_event_type("s3:TestEvent"), None);
        assert_eq!(map_event_type("unknown"), None);
        assert_eq!(map_event_type(""), None);
    }
}
