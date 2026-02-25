use std::collections::HashMap;

use async_trait::async_trait;
use celerity_runtime_core::auth_custom::{
  AuthGuardHandler, AuthGuardValidateError, AuthGuardValidateInput,
};
use pyo3::prelude::*;
use pythonize::pythonize;
use tokio::sync::mpsc;

use crate::interop::PythonCall;

// ---------------------------------------------------------------------------
// Python-facing input/output types
// ---------------------------------------------------------------------------

#[pyclass(name = "GuardInput")]
pub struct PyGuardInput {
  #[pyo3(get)]
  pub token: String,
  #[pyo3(get)]
  pub request: Py<PyGuardRequestInfo>,
  #[pyo3(get)]
  pub auth: Py<PyAny>,
  #[pyo3(get)]
  pub handler_name: Option<String>,
}

#[pyclass(name = "GuardRequestInfo")]
pub struct PyGuardRequestInfo {
  #[pyo3(get)]
  pub method: String,
  #[pyo3(get)]
  pub path: String,
  #[pyo3(get)]
  pub headers: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub query: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub cookies: HashMap<String, String>,
  #[pyo3(get)]
  pub body: Option<String>,
  #[pyo3(get)]
  pub request_id: String,
  #[pyo3(get)]
  pub client_ip: String,
}

#[pyclass(name = "GuardResult")]
pub struct PyGuardResult {
  #[pyo3(get, set)]
  pub status: String,
  #[pyo3(get, set)]
  pub auth: Option<Py<PyAny>>,
  #[pyo3(get, set)]
  pub message: Option<String>,
}

#[pymethods]
impl PyGuardResult {
  #[new]
  #[pyo3(signature = (status, auth=None, message=None))]
  fn new(status: String, auth: Option<Py<PyAny>>, message: Option<String>) -> Self {
    Self {
      status,
      auth,
      message,
    }
  }
}

// ---------------------------------------------------------------------------
// Conversion: core AuthGuardValidateInput → PyGuardInput
// ---------------------------------------------------------------------------

impl PyGuardInput {
  pub fn from_core(input: &AuthGuardValidateInput) -> PyResult<Self> {
    Python::with_gil(|py| {
      let headers = {
        let mut map: HashMap<String, Vec<String>> = HashMap::new();
        for (key, value) in input.request.headers.iter() {
          map
            .entry(key.as_str().to_string())
            .or_default()
            .push(value.to_str().unwrap_or_default().to_string());
        }
        map
      };

      let cookies = input
        .request
        .cookies
        .iter()
        .map(|c| (c.name().to_string(), c.value().to_string()))
        .collect();

      let request = Py::new(
        py,
        PyGuardRequestInfo {
          method: input.request.method.clone(),
          path: input.request.path.clone(),
          headers,
          query: input.request.query.clone(),
          cookies,
          body: input.request.body.clone(),
          request_id: input.request.request_id.0.clone(),
          client_ip: input.request.client_ip.clone(),
        },
      )?;

      let auth = pythonize(py, &serde_json::Value::Object(input.auth.clone()))?.unbind();

      Ok(Self {
        token: input.token.clone(),
        request,
        auth,
        handler_name: input.handler_name.clone(),
      })
    })
  }
}

// ---------------------------------------------------------------------------
// Conversion: PyGuardResult → core Result
// ---------------------------------------------------------------------------

fn py_obj_to_guard_result(
  py_obj: Py<PyAny>,
) -> PyResult<Result<serde_json::Value, AuthGuardValidateError>> {
  Python::with_gil(|py| {
    let bound = py_obj.bind(py);
    let result = bound.downcast::<PyGuardResult>()?.borrow();

    match result.status.as_str() {
      "allowed" => {
        let auth_value = match &result.auth {
          Some(auth_py) => {
            let bound_auth = auth_py.bind(py);
            pythonize::depythonize::<serde_json::Value>(bound_auth)
              .unwrap_or(serde_json::Value::Null)
          }
          None => serde_json::Value::Null,
        };
        Ok(Ok(auth_value))
      }
      "unauthorised" => Ok(Err(AuthGuardValidateError::Unauthorised(
        result.message.clone().unwrap_or_default(),
      ))),
      "forbidden" => Ok(Err(AuthGuardValidateError::Forbidden(
        result.message.clone().unwrap_or_default(),
      ))),
      _ => Ok(Err(AuthGuardValidateError::UnexpectedError(
        result
          .message
          .clone()
          .unwrap_or_else(|| "Guard validation failed".to_string()),
      ))),
    }
  })
}

// ---------------------------------------------------------------------------
// PyAuthGuardHandler — implements AuthGuardHandler via PythonCall dispatch
// ---------------------------------------------------------------------------

pub struct PyAuthGuardHandler {
  handler_id: String,
  py_tx: mpsc::UnboundedSender<PythonCall>,
}

impl PyAuthGuardHandler {
  pub fn new(handler_id: String, py_tx: mpsc::UnboundedSender<PythonCall>) -> Self {
    Self { handler_id, py_tx }
  }
}

impl std::fmt::Debug for PyAuthGuardHandler {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    f.debug_struct("PyAuthGuardHandler")
      .field("handler_id", &self.handler_id)
      .finish()
  }
}

// SAFETY: PyAuthGuardHandler only holds the py_tx sender (which is Send + Sync)
// and plain data. The Py<PyAny> handler references live in the shared handler_registry,
// not here.
unsafe impl Send for PyAuthGuardHandler {}
unsafe impl Sync for PyAuthGuardHandler {}

#[async_trait]
impl AuthGuardHandler for PyAuthGuardHandler {
  async fn validate(
    &self,
    input: AuthGuardValidateInput,
  ) -> Result<serde_json::Value, AuthGuardValidateError> {
    let py_input = PyGuardInput::from_core(&input)
      .map_err(|e| AuthGuardValidateError::UnexpectedError(e.to_string()))?;

    let py_input_obj =
      Python::with_gil(|py| Py::new(py, py_input).map(|p| p.into_any())).map_err(|e| {
        AuthGuardValidateError::UnexpectedError(format!("failed to create Python input: {e}"))
      })?;

    let (response_tx, response_rx) = tokio::sync::oneshot::channel();
    self
      .py_tx
      .send(PythonCall {
        handler_id: self.handler_id.clone(),
        args: vec![py_input_obj],
        response: response_tx,
      })
      .map_err(|_| {
        AuthGuardValidateError::UnexpectedError("Python worker unavailable".to_string())
      })?;

    let py_obj = response_rx
      .await
      .map_err(|_| AuthGuardValidateError::UnexpectedError("Python worker dropped".to_string()))?
      .map_err(|e| AuthGuardValidateError::UnexpectedError(e.to_string()))?;

    py_obj_to_guard_result(py_obj)
      .map_err(|e| AuthGuardValidateError::UnexpectedError(e.to_string()))?
  }
}
