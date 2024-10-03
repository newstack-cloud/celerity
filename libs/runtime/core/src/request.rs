use axum::{extract::Request, http::StatusCode, middleware::Next, response::Response};
use nanoid::nanoid;

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
        .map(|value| value.to_string())
        .unwrap_or_else(|| nanoid!());

    request.extensions_mut().insert(RequestId(req_id));

    let response = next.run(request).await;

    Ok(response)
}
