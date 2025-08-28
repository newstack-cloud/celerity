use std::collections::HashMap;

use axum::{
    body::Bytes,
    extract::{
        rejection::{PathRejection, QueryRejection},
        FromRequestParts, Path, Query,
    },
    http::{request::Parts, HeaderMap, HeaderValue, Uri},
};
use axum_extra::extract::CookieJar;

const DEFAULT_CONTENT_TYPE: &str = "application/octet-stream";

/// Converts the provided request body to an appropriate string
/// to be passed into a handler over an FFI boundary.
/// For text content types, the body is returned as a utf-8 encoded string,
/// for binary content types, the body is returned as a base64 encoded string.
pub fn to_request_body(
    body: &Bytes,
    content_type: Option<HeaderValue>,
) -> (Option<String>, Option<Vec<u8>>, String) {
    let content_type_str = content_type
        .as_ref()
        .and_then(|ct| ct.to_str().ok())
        .unwrap_or(DEFAULT_CONTENT_TYPE);

    if is_text_content_type(content_type_str) {
        // Try to decode as UTF-8
        match String::from_utf8(body.to_vec()) {
            Ok(text) => (Some(text), None, content_type_str.to_string()),
            Err(_) => {
                // UTF-8 decode failed, treat as binary even if the
                // content type suggests text.
                (None, Some(body.to_vec()), DEFAULT_CONTENT_TYPE.to_string())
            }
        }
    } else {
        (None, Some(body.to_vec()), content_type_str.to_string())
    }
}

fn is_text_content_type(content_type: &str) -> bool {
    let content_type_lower = content_type.to_lowercase();
    let base_type = content_type_lower
        .split(';')
        .next()
        .unwrap_or(&content_type_lower)
        .trim();

    base_type.starts_with("text/") 

    // Common text-based application types
    || base_type.starts_with("application/json")
    || base_type.starts_with("application/ld+json")
    || base_type.starts_with("application/xml")
    || base_type.starts_with("application/xhtml+xml")
    || base_type.starts_with("application/manifest+json")
    || base_type.starts_with("application/x-www-form-urlencoded")
    || base_type.starts_with("application/javascript")
    || base_type.starts_with("application/yaml")
    || base_type.starts_with("application/toml")
    || base_type.starts_with("application/csv")
    || base_type.starts_with("application/markdown")

    // Custom content types that are likely to be text
    || (base_type.starts_with("application/vnd.") &&
        (base_type.ends_with("+json")
         || base_type.ends_with("+xml")
         || base_type.ends_with("+text")))
}

/// Converts an axum header map to a hashmap of header names to lists of values,
/// supporting multiple values per header name.
pub fn headers_to_hashmap(headers: &HeaderMap) -> HashMap<String, Vec<String>> {
    let mut map = HashMap::<String, Vec<String>>::new();

    for (key, value) in headers.iter() {
        map.entry(key.to_string())
            .or_default()
            .push(value.to_str().unwrap_or("").to_string());
    }

    map
}

/// Converts an axum query map to a hashmap of query parameter names to lists of values,
/// supporting multiple values per parameter name.
pub fn query_from_uri(uri: &Uri) -> Result<HashMap<String, Vec<String>>, QueryRejection> {
    let query: Query<Vec<(String, String)>> = Query::try_from_uri(uri)?;
    let mut final_map = HashMap::<String, Vec<String>>::new();

    for (key, value) in query.0 {
        final_map.entry(key).or_default().push(value);
    }

    Ok(final_map)
}

/// Converts an axum cookie jar to a hashmap of cookie names to values,
/// supporting a single value per cookie name.
pub fn cookies_from_headers(headers: &HeaderMap) -> HashMap<String, String> {
    let cookies = CookieJar::from_headers(headers);
    let mut final_map = HashMap::<String, String>::new();

    for cookie in cookies.iter() {
        final_map.insert(cookie.name().to_string(), cookie.value().to_string());
    }

    final_map
}

/// Extracts the path parameters from axum request parts.
pub async fn path_params_from_request_parts(
    parts: &mut Parts,
) -> Result<HashMap<String, String>, PathRejection> {
    Path::<HashMap<String, String>>::from_request_parts(parts, &())
        .await
        .map(|path| path.0)
}

#[cfg(test)]
mod tests {
    use std::net::{Ipv4Addr, SocketAddr};

    use axum::{body::Body, http::Request, routing::get, Router};
    use http_body_util::BodyExt;
    use tokio::net::TcpListener;

    use super::*;

    #[test]
    fn test_headers_to_hashmap() {
        let mut input_headers = HeaderMap::new();
        input_headers.insert("Content-Type", HeaderValue::from_static("text/plain"));
        input_headers.append("Cookie", HeaderValue::from_static("cookie1=value1"));
        input_headers.append("Cookie", HeaderValue::from_static("cookie2=value2"));

        let output_map = headers_to_hashmap(&input_headers);

        assert_eq!(output_map.get("content-type").unwrap(), &vec!["text/plain"]);
        assert_eq!(
            output_map.get("cookie").unwrap(),
            &vec!["cookie1=value1", "cookie2=value2"]
        );
    }

    #[test]
    fn test_query_from_uri() {
        let uri = Uri::from_static("https://example.com?foo=bar&foo=baz&other=value");
        let query = query_from_uri(&uri);

        assert_eq!(
            query.unwrap(),
            HashMap::from([
                (
                    "foo".to_string(),
                    vec!["bar".to_string(), "baz".to_string()]
                ),
                ("other".to_string(), vec!["value".to_string()])
            ])
        );
    }

    #[test]
    fn test_cookies_from_headers() {
        let mut input_headers = HeaderMap::new();
        input_headers.insert(
            "Cookie",
            HeaderValue::from_static("cookie1=value1; cookie2=value2"),
        );
        input_headers.append("Cookie", HeaderValue::from_static("cookie3=value3"));

        let output_map = cookies_from_headers(&input_headers);

        assert_eq!(output_map.get("cookie1").unwrap(), &"value1".to_string());
        assert_eq!(output_map.get("cookie2").unwrap(), &"value2".to_string());
        assert_eq!(output_map.get("cookie3").unwrap(), &"value3".to_string());
    }

    #[test_log::test(tokio::test)]
    async fn test_path_params_from_request_parts() {
        // Path parameter extraction is deeply coupled with the axum router,
        // so we need to use a router to test extracting to a hashmap works correctly.
        let app = Router::new().route(
            "/users/{user_id}/posts/{post_id}",
            get(|req: Request<Body>| async move {
                let (mut parts, _) = req.into_parts();
                let path_params = path_params_from_request_parts(&mut parts).await.unwrap();
                serde_json::to_string(&path_params).unwrap()
            }),
        );

        // Set up a test server to run the router to handle the request.
        let listener = TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        // Test the actual path parameter extraction
        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();
        let response = client
            .request(
                Request::builder()
                    .method("GET")
                    .uri(format!("http://{addr}/users/123/posts/456"))
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        let body = response.into_body().collect().await.unwrap().to_bytes();
        let path_params: HashMap<String, String> = serde_json::from_slice(&body).unwrap();

        assert_eq!(
            path_params,
            HashMap::from([
                ("user_id".to_string(), "123".to_string()),
                ("post_id".to_string(), "456".to_string()),
            ])
        );
    }
}
