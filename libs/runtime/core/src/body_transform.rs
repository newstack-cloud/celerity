//! Provider-specific body transformers for consumer event messages.
//!
//! Each event source type (bucket, datastore, etc.) has a provider-specific raw
//! event format that must be normalised into the Celerity-standard shape before
//! reaching SDK handlers.  This module isolates that provider-specific logic so
//! the core consumer handler remains provider-agnostic.
//!
//! The `provider` parameter in the public dispatch functions determines which
//! provider implementation to use.  This value comes from the deploy target
//! environment — in local dev it reflects whichever cloud the application
//! targets (e.g. `"aws"` for AWS deployments, `"gcp"` for Google Cloud).
//!
//! When additional cloud providers are introduced, add a new provider module
//! and extend the match arms in [`transform_body`] and [`map_event_type`].

pub mod aws_dynamodb;
pub mod aws_s3;

// Future providers:
// pub mod gcp_firestore;
// pub mod gcp_gcs;

/// Transforms a raw event body into the Celerity-standard shape for the given
/// `source_type` and `provider`.
///
/// Returns `Some(transformed)` when a transform was applied, or `None` if no
/// transform is defined for the combination.
pub fn transform_body(provider: &str, source_type: &str, body: &str) -> Option<String> {
    match (provider, source_type) {
        ("aws", "bucket") => Some(aws_s3::transform(body)),
        ("aws", "datastore") => Some(aws_dynamodb::transform(body)),
        // ("gcp", "bucket") => Some(gcp_gcs::transform(body)),
        // ("gcp", "datastore") => Some(gcp_firestore::transform(body)),
        _ => None,
    }
}

/// Maps a provider-specific event name to a Celerity-standard event type
/// string (e.g. `"created"`, `"inserted"`).
///
/// Returns `None` when no mapping is defined for the `(provider, source_type)`
/// combination, or the event name is unrecognised.
pub fn map_event_type(
    provider: &str,
    event_name: &str,
    source_type: Option<&str>,
) -> Option<String> {
    match (provider, source_type) {
        ("aws", Some("bucket")) => aws_s3::map_event_type(event_name).map(String::from),
        ("aws", Some("datastore")) => aws_dynamodb::map_event_type(event_name).map(String::from),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::Value;

    #[test]
    fn returns_none_for_unknown_source_type() {
        assert!(transform_body("aws", "queue", r#"{"data":"test"}"#).is_none());
        assert!(transform_body("aws", "topic", r#"{"data":"test"}"#).is_none());
    }

    #[test]
    fn returns_none_for_unknown_provider() {
        let body = r#"{"Records":[{"s3":{"bucket":{"name":"b"},"object":{"key":"k"}}}]}"#;
        assert!(transform_body("gcp", "bucket", body).is_none());
        assert!(transform_body("unknown", "datastore", "{}").is_none());
    }

    #[test]
    fn dispatches_to_aws_bucket() {
        let body = r#"{"Records":[{"s3":{"bucket":{"name":"b"},"object":{"key":"k","size":1,"eTag":"e"}}}]}"#;
        let result = transform_body("aws", "bucket", body).unwrap();
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["key"], "k");
        assert_eq!(parsed["size"], 1);
        assert_eq!(parsed["eTag"], "e");
    }

    #[test]
    fn dispatches_to_aws_datastore() {
        let body = r#"{"Keys":{"id":{"S":"123"}},"NewImage":{"id":{"S":"123"}}}"#;
        let result = transform_body("aws", "datastore", body).unwrap();
        let parsed: Value = serde_json::from_str(&result).unwrap();
        assert_eq!(parsed["keys"]["id"], "123");
        assert_eq!(parsed["newItem"]["id"], "123");
    }

    #[test]
    fn map_event_type_dispatches_by_provider_and_source_type() {
        assert_eq!(
            map_event_type("aws", "s3:ObjectCreated:Put", Some("bucket")),
            Some("created".to_string())
        );
        assert_eq!(
            map_event_type("aws", "INSERT", Some("datastore")),
            Some("inserted".to_string())
        );
        assert_eq!(map_event_type("aws", "INSERT", Some("queue")), None);
        assert_eq!(map_event_type("aws", "INSERT", None), None);
        assert_eq!(
            map_event_type("gcp", "s3:ObjectCreated:Put", Some("bucket")),
            None
        );
    }
}
