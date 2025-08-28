use std::collections::HashMap;

use opentelemetry::global;
use tracing_opentelemetry::OpenTelemetrySpanExt;

/// Extracts the current trace context including a W3C Trace Context string
/// for a traceparent header and platform specific trace IDs such as an AWS X-Ray Trace ID.
pub fn extract_trace_context() -> Option<HashMap<String, String>> {
    let current_span = tracing::Span::current();
    let context = current_span.context();
    let mut carrier = std::collections::HashMap::new();
    global::get_text_map_propagator(|propagator| {
        propagator.inject_context(&context, &mut carrier);
    });

    let mut trace_context = HashMap::new();
    if let Some(traceparent) = carrier.get("traceparent") {
        trace_context.insert("traceparent".to_string(), traceparent.to_string());
    }
    if let Some(xray_trace_id) = carrier.get("X-Amzn-Trace-Id") {
        trace_context.insert("xray_trace_id".to_string(), xray_trace_id.to_string());
    }

    if !trace_context.is_empty() {
        Some(trace_context)
    } else {
        None
    }
}
