use std::{collections::HashMap, sync::Arc};

use pyo3::{prelude::*, types::PyTuple};
use tokio::sync::{mpsc, oneshot, Mutex as TokioMutex};
use tracing::info;

use crate::{errors::HandlerError, http::PyResponse};

/// Message sent to the Python worker responsible for calling python handlers
/// via the python asyncio event loop.
pub struct PythonCall {
  pub handler_id: String,
  pub args: Vec<PyObject>,
  pub response: oneshot::Sender<Result<PyResponse, HandlerError>>,
}

/// Starts the Python worker responsible for calling python handlers
/// via the python asyncio event loop.
pub async fn python_worker(
  mut rx: mpsc::UnboundedReceiver<PythonCall>,
  handlers: Arc<TokioMutex<HashMap<String, Py<PyAny>>>>,
) {
  let task_locals = Python::with_gil(pyo3_async_runtimes::tokio::get_current_locals)
    .expect("should be able to get task locals for current context");

  while let Some(call) = rx.recv().await {
    let handlers_clone = handlers.clone();

    // Propagate the task locals with the python event loop to the new task
    // so we can concurrently process python calls (for HTTP requests, WS messages and more)
    // while ensuring each task has access to the same python asyncio event loop.
    let task_locals_copy = Python::with_gil(|py| task_locals.clone_ref(py));
    tokio::spawn(pyo3_async_runtimes::tokio::scope(
      task_locals_copy,
      process_python_call(call, handlers_clone),
    ));
  }
}

async fn process_python_call(
  call: PythonCall,
  handlers: Arc<TokioMutex<HashMap<String, Py<PyAny>>>>,
) {
  let result = async {
    let handlers_lock = handlers.lock().await;
    let handler = handlers_lock.get(&call.handler_id);

    let wrapped_future = Python::with_gil(|py| {
      if let Some(handler) = handler {
        let internal_handler = handler.bind(py);
        let coro_args = PyTuple::new(py, call.args)?;
        let coro: Bound<'_, PyAny> = internal_handler.call1(coro_args)?;
        pyo3_async_runtimes::tokio::into_future(coro)
      } else {
        Err(PyErr::new::<pyo3::exceptions::PyException, _>(format!(
          "Handler not found: {}",
          call.handler_id,
        )))
      }
    });

    match wrapped_future {
      Ok(future) => {
        info!("Calling future wrapper for handler: {:?}", call.handler_id);
        let run_result = future
          .await
          .map_err(|err| HandlerError::new(err.to_string()));

        match run_result {
          Ok(output) => Python::with_gil(|py| -> PyResult<PyResponse> { output.extract(py) })
            .map_err(|err| HandlerError::new(err.to_string())),
          Err(err) => Err(err),
        }
      }
      Err(err) => Err(HandlerError::new(err.to_string())),
    }
  }
  .await;

  let _ = call.response.send(result);
}
