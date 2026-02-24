use std::collections::HashMap;

use opentelemetry::propagation::Extractor;

/// Provides a thin wrapper around the trace context HashMap
/// from a Redis stream message that implements the OpenTelemetry
/// text map propagator `Extractor` trait.
///
/// This allows extracting W3C `traceparent` headers or other
/// propagation headers (e.g. `x-amzn-trace-id`) from the message's
/// trace context to link consumer spans to upstream producer spans.
pub struct RedisMessageTraceContextExtractor<'a> {
    trace_context: &'a HashMap<String, String>,
}

impl<'a> RedisMessageTraceContextExtractor<'a> {
    pub fn new(trace_context: &'a HashMap<String, String>) -> Self {
        Self { trace_context }
    }
}

impl<'a> Extractor for RedisMessageTraceContextExtractor<'a> {
    fn get(&self, key: &str) -> Option<&str> {
        self.trace_context.get(key).map(|v| v.as_str())
    }

    fn keys(&self) -> Vec<&str> {
        self.trace_context.keys().map(|k| k.as_str()).collect()
    }
}
