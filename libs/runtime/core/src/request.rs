use std::collections::HashMap;

use axum::{
    extract::Request,
    http::{HeaderMap, StatusCode},
    middleware::Next,
    response::Response,
};
use axum_extra::extract::CookieJar;
use nanoid::nanoid;
use reqwest::Version;
use serde::{Deserialize, Serialize};

use crate::consts::REQUEST_ID_HEADER;

#[derive(Clone)]
pub struct RequestId(pub String);

// A middleware function for extracting a request ID from the request headers,
// falling back to generating one if not found.
pub async fn request_id(mut request: Request, next: Next) -> Result<Response, StatusCode> {
    let req_id = request
        .headers()
        .get(REQUEST_ID_HEADER)
        .and_then(|value| value.to_str().ok())
        .map_or_else(|| nanoid!(), |value| value.to_string());

    request.extensions_mut().insert(RequestId(req_id));

    let response = next.run(request).await;

    Ok(response)
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum HttpProtocolVersion {
    #[serde(rename = "HTTP1_1")]
    Http1_1,
    #[serde(rename = "HTTP2")]
    Http2,
    #[serde(rename = "HTTP3")]
    Http3,
}

impl From<Version> for HttpProtocolVersion {
    fn from(version: Version) -> Self {
        match version {
            Version::HTTP_2 => Self::Http2,
            Version::HTTP_3 => Self::Http3,
            // Any version before HTTP/1.1 is treated as HTTP/1.1,
            // this shouldn't cause any issues as typically, systems
            // making requests to Celerity apps should be using HTTP/1.1 or above.
            _ => Self::Http1_1,
        }
    }
}

/// A struct that provides information about the request that is being processed.
/// This is passed into handlers for APIs and custom auth guards.
pub struct RequestInfo {
    pub request_id: RequestId,
    pub headers: HeaderMap,
    pub query: HashMap<String, Vec<String>>,
    pub cookies: CookieJar,
    pub body: Option<String>,
    pub client_ip: String,
}
