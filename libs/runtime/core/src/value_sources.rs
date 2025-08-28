use std::{collections::HashMap, fmt::Display, str::FromStr};

use axum::http::HeaderMap;
use axum_extra::extract::CookieJar;
use jsonpath_rust::JsonPath;

#[derive(Debug)]
pub enum ExtractValueError {
    ValueSourceNotFound(String),
    ValueSourceInvalid(String),
    UnexpectedError(String),
}

impl Display for ExtractValueError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            ExtractValueError::ValueSourceNotFound(value_source) => {
                write!(f, "Value source not found: {value_source}")
            }
            ExtractValueError::ValueSourceInvalid(value_source) => {
                write!(f, "Value source invalid: {value_source}")
            }
            ExtractValueError::UnexpectedError(error) => write!(f, "Unexpected error: {error}"),
        }
    }
}

impl From<jsonpath_rust::JsonPathParserError> for ExtractValueError {
    fn from(error: jsonpath_rust::JsonPathParserError) -> Self {
        ExtractValueError::UnexpectedError(error.to_string())
    }
}

// Extracts values from HTTP request elements for HTTP requests
// including the `connect` (upgrade) phase of a WebSocket connection.
pub fn extract_value_from_request_elements(
    value_source: String,
    body: serde_json::Value,
    headers: &HeaderMap,
    query: &HashMap<String, Vec<String>>,
    cookies: &CookieJar,
) -> Result<serde_json::Value, ExtractValueError> {
    if value_source.starts_with("$.headers.") {
        return extract_value_from_headers(value_source, headers);
    }

    if value_source.starts_with("$.query.") {
        return extract_value_from_query(value_source, query);
    }

    if value_source.starts_with("$.cookies.") {
        return extract_value_from_cookies(value_source, cookies);
    }

    if value_source.starts_with("$.body.") {
        return extract_value_from_json_body(value_source, body);
    }

    Ok(serde_json::Value::Null)
}

fn extract_value_from_headers(
    value_source: String,
    headers: &HeaderMap,
) -> Result<serde_json::Value, ExtractValueError> {
    let header_name_opt = value_source.strip_prefix("$.headers.");
    if let Some(header_name) = header_name_opt {
        if let Some(header_value) = headers.get(header_name) {
            return Ok(serde_json::Value::String(
                header_value.to_str().unwrap_or_default().to_string(),
            ));
        }
    }

    Err(ExtractValueError::ValueSourceNotFound(value_source))
}

fn extract_value_from_query(
    value_source: String,
    query: &HashMap<String, Vec<String>>,
) -> Result<serde_json::Value, ExtractValueError> {
    let query_name_opt = value_source.strip_prefix("$.query.");
    if let Some(query_name) = query_name_opt {
        if let Some(query_value) = query.get(query_name) {
            // Usually, for a query parameter used as a value source,
            // only one value is expected.
            // For this reason, the first value from the list will
            // always be returned.
            if !query_value.is_empty() {
                return Ok(serde_json::Value::String(query_value[0].clone()));
            }
            return Err(ExtractValueError::ValueSourceNotFound(value_source));
        }
    }

    Err(ExtractValueError::ValueSourceNotFound(value_source))
}

fn extract_value_from_cookies(
    value_source: String,
    cookies: &CookieJar,
) -> Result<serde_json::Value, ExtractValueError> {
    let cookie_name_opt = value_source.strip_prefix("$.cookies.");
    if let Some(cookie_name) = cookie_name_opt {
        if let Some(cookie) = cookies.get(cookie_name) {
            return Ok(serde_json::Value::String(cookie.value().to_string()));
        }
    }

    Err(ExtractValueError::ValueSourceNotFound(value_source))
}

fn extract_value_from_json_body(
    value_source: String,
    body: serde_json::Value,
) -> Result<serde_json::Value, ExtractValueError> {
    let path_opt = value_source.strip_prefix("$.body.");
    if let Some(path) = path_opt {
        let json_path = JsonPath::from_str(format!("$.{path}").as_str())?;
        let result = json_path.find(&body);
        if let serde_json::Value::Array(result_data) = result {
            if !result_data.is_empty() {
                return Ok(result_data[0].clone());
            }
        }
    } else {
        return Err(ExtractValueError::ValueSourceInvalid(value_source));
    }

    Err(ExtractValueError::ValueSourceNotFound(value_source))
}

#[cfg(test)]
mod tests {
    use axum::http::{HeaderName, HeaderValue};
    use axum_extra::extract::cookie::Cookie;

    use super::*;

    #[test]
    fn test_extract_value_from_json_body() {
        let body = serde_json::json!({ "name": "John", "age": 30 });
        let value_source = "$.body.name".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            body,
            &HeaderMap::new(),
            &HashMap::new(),
            &CookieJar::new(),
        );
        assert!(result.is_ok());
        assert_eq!(
            result.unwrap(),
            serde_json::Value::String("John".to_string())
        );
    }

    #[test]
    fn test_fails_to_extract_value_from_json_body() {
        let body = serde_json::json!({ "name": "John", "age": 30 });
        let value_source = "$.body.not_found".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            body,
            &HeaderMap::new(),
            &HashMap::new(),
            &CookieJar::new(),
        );
        assert!(matches!(
            result,
            Err(ExtractValueError::ValueSourceNotFound(_))
        ));
    }

    #[test]
    fn test_extract_value_from_headers() {
        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("host"),
            HeaderValue::from_static("localhost:3000"),
        )]);
        let value_source = "$.headers.host".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &headers,
            &HashMap::new(),
            &CookieJar::new(),
        );
        assert!(result.is_ok());
        assert_eq!(
            result.unwrap(),
            serde_json::Value::String("localhost:3000".to_string())
        );
    }

    #[test]
    fn test_fails_to_extract_value_from_headers() {
        let headers = HeaderMap::new();
        let value_source = "$.headers.not-found".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &headers,
            &HashMap::new(),
            &CookieJar::new(),
        );
        assert!(matches!(
            result,
            Err(ExtractValueError::ValueSourceNotFound(_))
        ));
    }

    #[test]
    fn test_extract_value_from_query() {
        let query = HashMap::from_iter([("name".to_string(), vec!["John".to_string()])]);
        let value_source = "$.query.name".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &HeaderMap::new(),
            &query,
            &CookieJar::new(),
        );
        assert!(result.is_ok());
        assert_eq!(
            result.unwrap(),
            serde_json::Value::String("John".to_string())
        );
    }

    #[test]
    fn test_fails_to_extract_value_from_query() {
        let query = HashMap::new();
        let value_source = "$.query.not-found".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &HeaderMap::new(),
            &query,
            &CookieJar::new(),
        );
        assert!(matches!(
            result,
            Err(ExtractValueError::ValueSourceNotFound(_))
        ));
    }

    #[test]
    fn test_extract_value_from_cookies() {
        let cookies = CookieJar::new().add(Cookie::new("name", "John"));
        let value_source = "$.cookies.name".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &HeaderMap::new(),
            &HashMap::new(),
            &cookies,
        );
        assert!(result.is_ok());
        assert_eq!(
            result.unwrap(),
            serde_json::Value::String("John".to_string())
        );
    }

    #[test]
    fn test_fails_to_extract_value_from_cookies() {
        let cookies = CookieJar::new();
        let value_source = "$.cookies.not-found".to_string();
        let result = extract_value_from_request_elements(
            value_source,
            serde_json::Value::Null,
            &HeaderMap::new(),
            &HashMap::new(),
            &cookies,
        );
        assert!(matches!(
            result,
            Err(ExtractValueError::ValueSourceNotFound(_))
        ));
    }
}
