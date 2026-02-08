use std::collections::HashMap;

use axum::http::Version;
use axum::{body::Body, response::IntoResponse};
use celerity_runtime_core::config::{HttpConfig, HttpHandlerDefinition};
use celerity_runtime_core::request::HttpProtocolVersion as CoreHttpProtocolVersion;
use pyo3::prelude::*;
use pythonize::depythonize;

#[pyclass]
pub struct CoreHttpConfig {
  #[pyo3(get)]
  handlers: Vec<Py<CoreHttpHandlerDefinition>>,
}

pub fn core_http_config(http_config: HttpConfig) -> Py<CoreHttpConfig> {
  Python::with_gil(|py| Py::new(py, CoreHttpConfig::from(http_config)).unwrap())
}

impl From<HttpConfig> for CoreHttpConfig {
  fn from(http_config: HttpConfig) -> Self {
    let handlers = http_config
      .handlers
      .into_iter()
      .map(core_http_handler_definition)
      .collect::<Vec<_>>();
    Self { handlers }
  }
}

#[pyclass]
pub struct CoreHttpHandlerDefinition {
  #[pyo3(get)]
  path: String,
  #[pyo3(get)]
  method: String,
  #[pyo3(get)]
  location: String,
  #[pyo3(get)]
  handler: String,
  #[pyo3(get)]
  timeout: i64,
}

pub fn core_http_handler_definition(
  http_handler_definition: HttpHandlerDefinition,
) -> Py<CoreHttpHandlerDefinition> {
  Python::with_gil(|py| {
    Py::new(py, CoreHttpHandlerDefinition::from(http_handler_definition)).unwrap()
  })
}

impl From<HttpHandlerDefinition> for CoreHttpHandlerDefinition {
  fn from(handler: HttpHandlerDefinition) -> Self {
    Self {
      path: handler.path,
      method: handler.method,
      location: handler.location,
      handler: handler.handler,
      timeout: handler.timeout,
    }
  }
}

#[pyclass(name = "Response")]
#[derive(Debug, Clone)]
pub struct PyResponse {
  #[pyo3(get)]
  status: u16,
  #[pyo3(get)]
  headers: HashMap<String, String>,
  #[pyo3(get)]
  text_body: Option<String>,
  #[pyo3(get)]
  binary_body: Option<Vec<u8>>,
}

impl IntoResponse for PyResponse {
  fn into_response(self) -> axum::response::Response<Body> {
    let mut builder = axum::response::Response::builder();
    for (key, value) in self.headers {
      builder = builder.header(key, value);
    }
    let body = if let Some(text_body) = self.text_body {
      Body::from(text_body)
    } else if let Some(binary_body) = self.binary_body {
      Body::from(binary_body)
    } else {
      Body::from("")
    };
    builder = builder.status(self.status);
    builder.body(body).unwrap()
  }
}

struct InternalPyResponse {
  status: u16,
  headers: Option<HashMap<String, String>>,
  text_body: Option<String>,
  json_body: Option<Py<PyAny>>,
  binary_body: Option<Vec<u8>>,
}

#[pyclass(name = "ResponseBuilder")]
pub struct PyResponseBuilder {
  response: InternalPyResponse,
}

#[pymethods]
impl PyResponseBuilder {
  #[new]
  fn new() -> Self {
    Self {
      response: InternalPyResponse {
        status: 200,
        headers: Some(HashMap::new()),
        text_body: None,
        json_body: None,
        binary_body: None,
      },
    }
  }

  fn set_status(mut self_: PyRefMut<Self>, status: u16) -> Py<Self> {
    self_.response.status = status;
    self_.into()
  }

  fn set_headers(mut self_: PyRefMut<Self>, headers: HashMap<String, String>) -> Py<Self> {
    self_.response.headers = Some(headers);
    self_.into()
  }

  fn set_text_body(mut self_: PyRefMut<Self>, text_body: String) -> Py<Self> {
    self_.response.text_body = Some(text_body);
    if let Some(headers) = &mut self_.response.headers {
      if !headers.contains_key("content-type") {
        headers.insert("content-type".to_string(), "text/plain".to_string());
      }
    }

    self_.into()
  }

  fn set_json_body(
    mut self_: PyRefMut<Self>,
    json_body: Py<PyAny>,
    py: Python,
  ) -> PyResult<Py<Self>> {
    let bound_ref = json_body.bind(py);
    let value: serde_json::Value = depythonize(bound_ref)?;
    self_.response.text_body = Some(serde_json::to_string(&value).unwrap());

    if let Some(headers) = &mut self_.response.headers {
      if !headers.contains_key("content-type") {
        headers.insert("content-type".to_string(), "application/json".to_string());
      }
    }

    Ok(self_.into())
  }

  fn set_binary_body(mut self_: PyRefMut<Self>, binary_body: Vec<u8>) -> Py<Self> {
    self_.response.binary_body = Some(binary_body);

    if let Some(headers) = &mut self_.response.headers {
      if !headers.contains_key("content-type") {
        headers.insert(
          "content-type".to_string(),
          "application/octet-stream".to_string(),
        );
      }
    }

    self_.into()
  }

  fn build(mut self_: PyRefMut<Self>, py: Python) -> PyResult<Py<PyResponse>> {
    let text_body = if let Some(json_body) = &self_.response.json_body {
      let bound_ref = json_body.bind(py);
      let value: serde_json::Value = depythonize(bound_ref)?;
      Some(serde_json::to_string(&value).map_err(|err| {
        PyErr::new::<pyo3::exceptions::PyValueError, _>(format!(
          "failed to convert JSON body to string, {err}",
        ))
      })?)
    } else {
      self_.response.text_body.take()
    };

    Py::new(
      py,
      PyResponse {
        status: self_.response.status,
        headers: self_.response.headers.take().unwrap_or_default(),
        text_body,
        binary_body: self_.response.binary_body.take(),
      },
    )
  }
}

#[pyclass(name = "Request")]
pub struct PyRequest {
  #[pyo3(get)]
  pub text_body: Option<String>,
  #[pyo3(get)]
  pub binary_body: Option<Vec<u8>>,
  #[pyo3(get)]
  pub content_type: String,
  #[pyo3(get)]
  pub headers: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub query: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub cookies: HashMap<String, String>,
  #[pyo3(get)]
  pub method: String,
  #[pyo3(get)]
  pub path: String,
  #[pyo3(get)]
  pub path_params: HashMap<String, String>,
  #[pyo3(get)]
  pub protocol_version: HttpProtocolVersion,
  #[pyo3(get)]
  pub user_agent: String,
}

impl Default for PyRequest {
  fn default() -> Self {
    Self {
      text_body: None,
      binary_body: None,
      content_type: "text/plain".to_string(),
      headers: HashMap::new(),
      query: HashMap::new(),
      cookies: HashMap::new(),
      method: "GET".to_string(),
      path: "/".to_string(),
      path_params: HashMap::new(),
      protocol_version: HttpProtocolVersion::Http1_1,
      user_agent: String::new(),
    }
  }
}

#[pyclass(name = "RequestBuilder")]
pub struct PyRequestBuilder {
  request: Option<PyRequest>,
}

#[pymethods]
impl PyRequestBuilder {
  #[new]
  fn new() -> Self {
    Self {
      request: Some(PyRequest::default()),
    }
  }

  fn set_text_body(mut self_: PyRefMut<Self>, text_body: String) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.text_body = Some(text_body);
      if request.content_type.is_empty() {
        request.content_type = "text/plain".to_string();
      }
    }
    self_.into()
  }

  fn set_binary_body(mut self_: PyRefMut<Self>, binary_body: Vec<u8>) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.binary_body = Some(binary_body);
      if request.content_type.is_empty() {
        request.content_type = "application/octet-stream".to_string();
      }
    }
    self_.into()
  }

  fn set_content_type(mut self_: PyRefMut<Self>, content_type: String) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.content_type = content_type;
    }
    self_.into()
  }

  fn set_headers(mut self_: PyRefMut<Self>, headers: HashMap<String, Vec<String>>) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.headers = headers;
    }
    self_.into()
  }

  fn set_query(mut self_: PyRefMut<Self>, query: HashMap<String, Vec<String>>) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.query = query;
    }
    self_.into()
  }

  fn set_cookies(mut self_: PyRefMut<Self>, cookies: HashMap<String, String>) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.cookies = cookies;
    }
    self_.into()
  }

  fn set_method(mut self_: PyRefMut<Self>, method: String) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.method = method;
    }
    self_.into()
  }

  fn set_path(mut self_: PyRefMut<Self>, path: String) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.path = path;
    }
    self_.into()
  }

  fn set_path_params(mut self_: PyRefMut<Self>, path_params: HashMap<String, String>) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.path_params = path_params;
    }
    self_.into()
  }

  fn set_protocol_version(
    mut self_: PyRefMut<Self>,
    protocol_version: HttpProtocolVersion,
  ) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.protocol_version = protocol_version;
    }
    self_.into()
  }

  fn set_user_agent(mut self_: PyRefMut<Self>, user_agent: String) -> Py<Self> {
    if let Some(request) = &mut self_.request {
      request.user_agent = user_agent;
    }
    self_.into()
  }

  fn build(mut self_: PyRefMut<Self>, py: Python) -> PyResult<Py<PyRequest>> {
    if let Some(request) = &mut self_.request {
      if request.content_type.is_empty() {
        let default_content_type_header = vec!["text/plain".to_string()];
        let header_vals = request
          .headers
          .get("content-type")
          .unwrap_or(&default_content_type_header);
        if !header_vals.is_empty() {
          request.content_type = header_vals[0].clone();
        }
      }
    }
    Py::new(py, self_.request.take().unwrap_or_default())
  }
}

#[pyclass(eq, eq_int, name = "HttpProtocolVersion")]
#[derive(PartialEq, Clone, Debug, Default)]
pub enum HttpProtocolVersion {
  #[pyo3(name = "HTTP1_1")]
  #[default]
  Http1_1,
  #[pyo3(name = "HTTP2")]
  Http2,
  #[pyo3(name = "HTTP3")]
  Http3,
}

impl From<CoreHttpProtocolVersion> for HttpProtocolVersion {
  fn from(protocol_version: CoreHttpProtocolVersion) -> Self {
    match protocol_version {
      CoreHttpProtocolVersion::Http1_1 => Self::Http1_1,
      CoreHttpProtocolVersion::Http2 => Self::Http2,
      CoreHttpProtocolVersion::Http3 => Self::Http3,
    }
  }
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

#[pyclass(name = "RequestContext")]
pub struct PyRequestContext {
  #[pyo3(get)]
  pub request_id: String,
  #[pyo3(get)]
  pub request_time: chrono::DateTime<chrono::Utc>,
  #[pyo3(get)]
  pub auth: Py<PyAny>,
  #[pyo3(get)]
  pub trace_context: Option<HashMap<String, String>>,
  #[pyo3(get)]
  pub client_ip: String,
  #[pyo3(get)]
  pub matched_route: Option<String>,
}

#[pymethods]
impl PyRequestContext {
  #[new]
  #[pyo3(signature = (request_id, request_time, auth, trace_context, client_ip, matched_route))]
  fn new(
    request_id: String,
    request_time: chrono::DateTime<chrono::Utc>,
    auth: Py<PyAny>,
    trace_context: Option<HashMap<String, String>>,
    client_ip: String,
    matched_route: Option<String>,
  ) -> Self {
    Self {
      request_id,
      request_time,
      auth,
      trace_context,
      client_ip,
      matched_route,
    }
  }
}
