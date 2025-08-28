use aws_sdk_sqs::types::MessageSystemAttributeName;
use celerity_helpers::aws_telemetry::AWS_XRAY_TRACE_HEADER_NAME;
use opentelemetry::propagation::Extractor;

use crate::types::SQSMessageMetadata;

/// Provides a thin wrapper around SQS message attributes
/// that can implement a text map propagator Extractor
/// to enrich trace spans with X-Ray trace IDs
/// provided in received SQS messages.
pub struct SQSMessageMetadataExtractor<'a> {
    metadata: &'a SQSMessageMetadata,
}

impl<'a> SQSMessageMetadataExtractor<'a> {
    pub fn new(metadata: &'a SQSMessageMetadata) -> Self {
        Self { metadata }
    }
}

impl<'a> Extractor for SQSMessageMetadataExtractor<'a> {
    /// Get a value for a key from the HashMap.
    fn get(&self, key: &str) -> Option<&str> {
        // This isn't ideal but it is the only way to reuse the X-Ray text map propagator
        // instead of rolling our own as it only recognises the HTTP header name
        // expecting to be extracting context from a HTTP request.
        let key_lowercase = key.to_lowercase();
        if key_lowercase == AWS_XRAY_TRACE_HEADER_NAME {
            if let Some(attributes) = &self.metadata.attributes {
                return attributes
                    .get(&MessageSystemAttributeName::AwsTraceHeader)
                    .map(|v| v.as_str());
            }
        }

        Some("")
    }

    /// Collect all the relevant keys from the underlying message metadata.
    fn keys(&self) -> Vec<&str> {
        let mut keys = Vec::new();

        if let Some(attributes) = &self.metadata.attributes {
            if attributes.contains_key(&MessageSystemAttributeName::AwsTraceHeader) {
                keys.push(AWS_XRAY_TRACE_HEADER_NAME);
            }
        }

        keys
    }
}
