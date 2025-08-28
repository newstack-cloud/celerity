use pyo3::prelude::*;
use pythonize::pythonize;
use serde_json::Value;

/// Converts a serde_json::Value to a Python object,
/// useful for WebSocket message conversion
/// to be passed into application handlers without having
/// to re-serialise and re-parse the body on the Python side.
pub fn json_value_to_python<'a>(value: &'a Value, py: Python<'a>) -> PyResult<Bound<'a, PyAny>> {
  Ok(pythonize(py, value)?)
}
