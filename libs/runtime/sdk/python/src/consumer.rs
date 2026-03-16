use std::{collections::HashMap, time::Duration};

use async_trait::async_trait;
use celerity_runtime_core::{
  consumer_handler::{ConsumerEventHandler, ConsumerEventHandlerError},
  telemetry_utils::extract_trace_context,
  types::{
    ConsumerEventData, EventResult, EventResultData, MessageProcessingFailure,
    MessageProcessingResponseData, ScheduleEventData, ScheduledEventResponseData,
  },
};
use pyo3::prelude::*;
use pythonize::pythonize;
use tokio::{sync::mpsc, time};

use crate::interop::PythonCall;

// ---------------------------------------------------------------------------
// Python-facing input/output types
// ---------------------------------------------------------------------------

#[pyclass(name = "ConsumerEventInput")]
pub struct PyConsumerEventInput {
  #[pyo3(get)]
  pub handler_tag: String,
  #[pyo3(get)]
  pub messages: Vec<Py<PyConsumerMessage>>,
  #[pyo3(get)]
  pub vendor: Py<PyAny>,
  #[pyo3(get)]
  pub trace_context: Option<HashMap<String, String>>,
}

#[pyclass(name = "ConsumerMessage")]
pub struct PyConsumerMessage {
  #[pyo3(get)]
  pub message_id: String,
  #[pyo3(get)]
  pub body: String,
  #[pyo3(get)]
  pub source: String,
  /// The type of source parsed from the `celerity:{type}:{name}` format.
  #[pyo3(get)]
  pub source_type: Option<String>,
  /// The name of the source parsed from the `celerity:{type}:{name}` format.
  #[pyo3(get)]
  pub source_name: Option<String>,
  /// The Celerity-standard event type (e.g. "created", "inserted").
  #[pyo3(get)]
  pub event_type: Option<String>,
  #[pyo3(get)]
  pub message_attributes: Py<PyAny>,
  #[pyo3(get)]
  pub vendor: Py<PyAny>,
}

#[pyclass(name = "ScheduleEventInput")]
pub struct PyScheduleEventInput {
  #[pyo3(get)]
  pub handler_tag: String,
  #[pyo3(get)]
  pub schedule_id: String,
  #[pyo3(get)]
  pub message_id: String,
  #[pyo3(get)]
  pub schedule: String,
  #[pyo3(get)]
  pub input: Option<Py<PyAny>>,
  #[pyo3(get)]
  pub vendor: Py<PyAny>,
  #[pyo3(get)]
  pub trace_context: Option<HashMap<String, String>>,
}

#[pyclass(name = "EventResult")]
pub struct PyEventResult {
  #[pyo3(get, set)]
  pub success: bool,
  #[pyo3(get, set)]
  pub failures: Option<Vec<Py<PyMessageProcessingFailure>>>,
  #[pyo3(get, set)]
  pub error_message: Option<String>,
}

#[pymethods]
impl PyEventResult {
  #[new]
  #[pyo3(signature = (success, failures=None, error_message=None))]
  fn new(
    success: bool,
    failures: Option<Vec<Py<PyMessageProcessingFailure>>>,
    error_message: Option<String>,
  ) -> Self {
    Self {
      success,
      failures,
      error_message,
    }
  }
}

#[pyclass(name = "MessageProcessingFailure")]
pub struct PyMessageProcessingFailure {
  #[pyo3(get, set)]
  pub message_id: String,
  #[pyo3(get, set)]
  pub error_message: Option<String>,
}

#[pymethods]
impl PyMessageProcessingFailure {
  #[new]
  #[pyo3(signature = (message_id, error_message=None))]
  fn new(message_id: String, error_message: Option<String>) -> Self {
    Self {
      message_id,
      error_message,
    }
  }
}

// ---------------------------------------------------------------------------
// Conversions: core types → Python types
// ---------------------------------------------------------------------------

impl PyConsumerEventInput {
  pub fn from_core(handler_tag: &str, event_data: ConsumerEventData) -> PyResult<Self> {
    Python::with_gil(|py| {
      let messages = event_data
        .messages
        .into_iter()
        .map(|m| {
          let message_attributes = pythonize(py, &m.message_attributes)?.unbind();
          let vendor = pythonize(py, &m.vendor)?.unbind();
          Py::new(
            py,
            PyConsumerMessage {
              message_id: m.message_id,
              body: m.body,
              source: m.source,
              source_type: m.source_type,
              source_name: m.source_name,
              event_type: m.event_type,
              message_attributes,
              vendor,
            },
          )
        })
        .collect::<PyResult<Vec<_>>>()?;

      let vendor = pythonize(py, &event_data.vendor)?.unbind();

      Ok(Self {
        handler_tag: handler_tag.to_string(),
        messages,
        vendor,
        trace_context: extract_trace_context(),
      })
    })
  }
}

impl PyScheduleEventInput {
  pub fn from_core(handler_tag: &str, event_data: ScheduleEventData) -> PyResult<Self> {
    Python::with_gil(|py| {
      let vendor = pythonize(py, &event_data.vendor)?.unbind();
      let input = event_data
        .input
        .as_ref()
        .map(|v| pythonize(py, v).map(|b| b.unbind()))
        .transpose()?;

      Ok(Self {
        handler_tag: handler_tag.to_string(),
        schedule_id: event_data.schedule_id,
        message_id: event_data.message_id,
        schedule: event_data.schedule,
        input,
        vendor,
        trace_context: extract_trace_context(),
      })
    })
  }
}

// ---------------------------------------------------------------------------
// Conversions: Python result → core EventResult
// ---------------------------------------------------------------------------

fn py_obj_to_consumer_event_result(py_obj: Py<PyAny>, event_id: &str) -> PyResult<EventResult> {
  Python::with_gil(|py| {
    let bound = py_obj.bind(py);
    let result = bound.downcast::<PyEventResult>()?.borrow();
    let failures = result.failures.as_ref().map(|fs| {
      fs.iter()
        .map(|f| {
          let f_ref = f.borrow(py);
          MessageProcessingFailure {
            message_id: f_ref.message_id.clone(),
            error_message: f_ref.error_message.clone(),
          }
        })
        .collect()
    });
    Ok(EventResult {
      event_id: event_id.to_string(),
      data: EventResultData::MessageProcessingResponse(MessageProcessingResponseData {
        success: result.success,
        failures,
      }),
      context: None,
    })
  })
}

fn py_obj_to_schedule_event_result(py_obj: Py<PyAny>, event_id: &str) -> PyResult<EventResult> {
  Python::with_gil(|py| {
    let bound = py_obj.bind(py);
    let result = bound.downcast::<PyEventResult>()?.borrow();
    Ok(EventResult {
      event_id: event_id.to_string(),
      data: EventResultData::ScheduledEventResponse(ScheduledEventResponseData {
        success: result.success,
        error_message: result.error_message.clone(),
      }),
      context: None,
    })
  })
}

// ---------------------------------------------------------------------------
// PyConsumerEventHandlerBuilder — accumulates per-handler_tag registrations
// ---------------------------------------------------------------------------

pub struct PyConsumerEventHandlerBuilder {
  consumer_handlers: HashMap<String, u64>,
  schedule_handlers: HashMap<String, u64>,
}

impl Default for PyConsumerEventHandlerBuilder {
  fn default() -> Self {
    Self::new()
  }
}

impl PyConsumerEventHandlerBuilder {
  pub fn new() -> Self {
    Self {
      consumer_handlers: HashMap::new(),
      schedule_handlers: HashMap::new(),
    }
  }

  pub fn add_consumer_handler(&mut self, handler_name: String, timeout_secs: u64) {
    self.consumer_handlers.insert(handler_name, timeout_secs);
  }

  pub fn add_schedule_handler(&mut self, handler_name: String, timeout_secs: u64) {
    self.schedule_handlers.insert(handler_name, timeout_secs);
  }

  pub fn is_empty(&self) -> bool {
    self.consumer_handlers.is_empty() && self.schedule_handlers.is_empty()
  }

  pub fn build(self, py_tx: mpsc::UnboundedSender<PythonCall>) -> PyConsumerEventHandler {
    PyConsumerEventHandler {
      consumer_handlers: self.consumer_handlers,
      schedule_handlers: self.schedule_handlers,
      py_tx,
    }
  }
}

// ---------------------------------------------------------------------------
// PyConsumerEventHandler — dispatches by handler_tag
// ---------------------------------------------------------------------------

pub struct PyConsumerEventHandler {
  consumer_handlers: HashMap<String, u64>,
  schedule_handlers: HashMap<String, u64>,
  py_tx: mpsc::UnboundedSender<PythonCall>,
}

// SAFETY: PyConsumerEventHandler only holds the py_tx sender (which is Send + Sync)
// and plain data. The Py<PyAny> handler references live in the shared handler_registry,
// not here.
unsafe impl Send for PyConsumerEventHandler {}
unsafe impl Sync for PyConsumerEventHandler {}

#[async_trait]
impl ConsumerEventHandler for PyConsumerEventHandler {
  async fn handle_consumer_event(
    &self,
    handler_tag: &str,
    event_data: ConsumerEventData,
  ) -> Result<EventResult, ConsumerEventHandlerError> {
    // handler_tag format: "source::<source_id>::<handler_name>"
    let handler_name = handler_tag.rsplit("::").next().unwrap_or(handler_tag);
    let timeout_secs = self
      .consumer_handlers
      .get(handler_name)
      .ok_or(ConsumerEventHandlerError::MissingHandler)?;

    let py_input = PyConsumerEventInput::from_core(handler_tag, event_data)
      .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;

    let handler_id = format!("consumer::{handler_name}");
    let py_input_obj =
      Python::with_gil(|py| Py::new(py, py_input).map(|p| p.into_any())).map_err(|e| {
        ConsumerEventHandlerError::HandlerFailure(format!("failed to create Python input: {e}"))
      })?;

    let (response_tx, response_rx) = tokio::sync::oneshot::channel();
    self
      .py_tx
      .send(PythonCall {
        handler_id,
        args: vec![py_input_obj],
        response: response_tx,
      })
      .map_err(|_| ConsumerEventHandlerError::ChannelClosed)?;

    let sleep = time::sleep(Duration::from_secs(*timeout_secs));
    tokio::select! {
      _ = sleep => {
        Err(ConsumerEventHandlerError::Timeout)
      }
      result = response_rx => {
        let py_obj = result
          .map_err(|_| ConsumerEventHandlerError::ChannelClosed)?
          .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;
        py_obj_to_consumer_event_result(py_obj, handler_tag)
          .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))
      }
    }
  }

  async fn handle_schedule_event(
    &self,
    handler_tag: &str,
    event_data: ScheduleEventData,
  ) -> Result<EventResult, ConsumerEventHandlerError> {
    // handler_tag format: "source::<schedule_id>::<handler_name>"
    let handler_name = handler_tag.rsplit("::").next().unwrap_or(handler_tag);
    let timeout_secs = self
      .schedule_handlers
      .get(handler_name)
      .ok_or(ConsumerEventHandlerError::MissingHandler)?;

    let py_input = PyScheduleEventInput::from_core(handler_tag, event_data)
      .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;

    let handler_id = format!("schedule::{handler_name}");
    let py_input_obj =
      Python::with_gil(|py| Py::new(py, py_input).map(|p| p.into_any())).map_err(|e| {
        ConsumerEventHandlerError::HandlerFailure(format!("failed to create Python input: {e}"))
      })?;

    let (response_tx, response_rx) = tokio::sync::oneshot::channel();
    self
      .py_tx
      .send(PythonCall {
        handler_id,
        args: vec![py_input_obj],
        response: response_tx,
      })
      .map_err(|_| ConsumerEventHandlerError::ChannelClosed)?;

    let sleep = time::sleep(Duration::from_secs(*timeout_secs));
    tokio::select! {
      _ = sleep => {
        Err(ConsumerEventHandlerError::Timeout)
      }
      result = response_rx => {
        let py_obj = result
          .map_err(|_| ConsumerEventHandlerError::ChannelClosed)?
          .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))?;
        py_obj_to_schedule_event_result(py_obj, handler_tag)
          .map_err(|e| ConsumerEventHandlerError::HandlerFailure(e.to_string()))
      }
    }
  }
}
