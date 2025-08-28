use std::fmt;

use opentelemetry::trace::TraceId;

const AWS_XRAY_VERSION_KEY: &str = "1";

pub const AWS_XRAY_TRACE_HEADER_NAME: &str = "x-amzn-trace-id";

/// Holds an X-Ray formatted Trace ID
///
/// (copied from opentelemetry_aws as the crate does not export it)
///
/// A `trace_id` consists of three numbers separated by hyphens. For example, `1-58406520-a006649127e371903a2de979`.
/// This includes:
///
/// * The version number, that is, 1.
/// * The time of the original request, in Unix epoch time, in 8 hexadecimal digits.
/// * For example, 10:00AM December 1st, 2016 PST in epoch time is 1480615200 seconds, or 58406520 in hexadecimal digits.
/// * A 96-bit identifier for the trace, globally unique, in 24 hexadecimal digits.
///
/// See the [AWS X-Ray Documentation][xray-trace-id] for more details.
///
/// [xray-trace-id]: https://docs.aws.amazon.com/xray/latest/devguide/xray-api-sendingdata.html#xray-api-traceids
#[derive(Clone, Debug, PartialEq)]
pub struct XrayTraceId(String);

impl TryFrom<XrayTraceId> for TraceId {
    type Error = ();

    fn try_from(id: XrayTraceId) -> Result<Self, Self::Error> {
        let parts: Vec<&str> = id.0.split_terminator('-').collect();

        if parts.len() != 3 {
            return Err(());
        }

        let trace_id: TraceId =
            TraceId::from_hex(format!("{}{}", parts[1], parts[2]).as_str()).map_err(|_| ())?;

        if trace_id == TraceId::INVALID {
            Err(())
        } else {
            Ok(trace_id)
        }
    }
}

impl From<TraceId> for XrayTraceId {
    fn from(trace_id: TraceId) -> Self {
        let trace_id_as_hex = trace_id.to_string();
        let (timestamp, xray_id) = trace_id_as_hex.split_at(8_usize);

        XrayTraceId(format!("{AWS_XRAY_VERSION_KEY}-{timestamp}-{xray_id}"))
    }
}

impl fmt::Display for XrayTraceId {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}
