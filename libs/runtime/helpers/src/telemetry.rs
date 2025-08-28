/// The key for the Celerity context ID that is used as a correlation ID
/// across async boundaries where provider trace IDs can not be propagated.
/// For example, when SNS and SQS are combined to deliver messages from a publisher
/// to consumers, the AWS X-Ray trace ID is not propagated to the SQS message consumer.
/// The Celerity context ID is embedded in the SNS message attributes and is used to correlate
/// the publisher and consumer spans.
pub const CELERITY_CONTEXT_ID_KEY: &str = "celerity.context-id";

/// The key for the Celerity context IDs that are used as a correlation ID
/// across async boundaries where provider trace IDs can not be propagated.
/// This is used to correlate spans for batch operations where a correlation ID
/// will be associated with each message being processed.
pub const CELERITY_CONTEXT_IDS_KEY: &str = "celerity.context-ids";
