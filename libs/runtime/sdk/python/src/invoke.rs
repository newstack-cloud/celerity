use std::time::Duration;

use async_trait::async_trait;
use celerity_runtime_core::handler_invoke::{HandlerInvokeError, HandlerInvoker};
use pyo3::prelude::*;
use pythonize::{depythonize, pythonize};
use tokio::{sync::mpsc, time};

use crate::interop::PythonCall;

// ---------------------------------------------------------------------------
// PyHandlerInvoker — implements HandlerInvoker via PythonCall dispatch
// ---------------------------------------------------------------------------

pub struct PyHandlerInvoker {
  handler_id: String,
  py_tx: mpsc::UnboundedSender<PythonCall>,
  timeout_secs: u64,
}

impl PyHandlerInvoker {
  pub fn new(
    handler_id: String,
    py_tx: mpsc::UnboundedSender<PythonCall>,
    timeout_secs: u64,
  ) -> Self {
    Self {
      handler_id,
      py_tx,
      timeout_secs,
    }
  }
}

#[async_trait]
impl HandlerInvoker for PyHandlerInvoker {
  async fn invoke(
    &self,
    payload: serde_json::Value,
  ) -> Result<serde_json::Value, HandlerInvokeError> {
    let py_payload =
      Python::with_gil(|py| pythonize(py, &payload).map(|b| b.unbind())).map_err(|e| {
        HandlerInvokeError::InvocationFailed(format!("failed to pythonize payload: {e}"))
      })?;

    let (response_tx, response_rx) = tokio::sync::oneshot::channel();
    self
      .py_tx
      .send(PythonCall {
        handler_id: self.handler_id.clone(),
        args: vec![py_payload],
        response: response_tx,
      })
      .map_err(|_| HandlerInvokeError::InvocationFailed("Python worker unavailable".to_string()))?;

    let sleep = time::sleep(Duration::from_secs(self.timeout_secs));
    tokio::select! {
      _ = sleep => {
        Err(HandlerInvokeError::InvocationFailed("handler timed out".to_string()))
      }
      result = response_rx => {
        let py_obj = result
          .map_err(|_| HandlerInvokeError::InvocationFailed("Python worker dropped".to_string()))?
          .map_err(|e| HandlerInvokeError::InvocationFailed(e.to_string()))?;
        Python::with_gil(|py| {
          let bound = py_obj.bind(py);
          depythonize::<serde_json::Value>(bound)
        })
        .map_err(|e| HandlerInvokeError::InvocationFailed(format!("failed to depythonize result: {e}")))
      }
    }
  }
}
