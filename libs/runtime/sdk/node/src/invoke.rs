use std::{sync::Arc, time::Duration};

use async_trait::async_trait;
use celerity_runtime_core::handler_invoke::{HandlerInvokeError, HandlerInvoker};
use napi::bindgen_prelude::*;
use napi::threadsafe_function::ThreadsafeFunction;
use tokio::time;

// ---------------------------------------------------------------------------
// ThreadsafeFunction type alias
// ---------------------------------------------------------------------------

pub(crate) type InvokeWeakTsfn = ThreadsafeFunction<
  serde_json::Value,
  Promise<serde_json::Value>,
  serde_json::Value,
  Status,
  true,
  true,
>;

// ---------------------------------------------------------------------------
// NapiHandlerInvoker — bridges handler invocation to JS via tsfn
// ---------------------------------------------------------------------------

pub struct NapiHandlerInvoker {
  tsfn: Arc<InvokeWeakTsfn>,
  timeout_secs: u64,
}

impl NapiHandlerInvoker {
  pub fn new(tsfn: Arc<InvokeWeakTsfn>, timeout_secs: u64) -> Self {
    Self { tsfn, timeout_secs }
  }
}

#[async_trait]
impl HandlerInvoker for NapiHandlerInvoker {
  async fn invoke(
    &self,
    payload: serde_json::Value,
  ) -> std::result::Result<serde_json::Value, HandlerInvokeError> {
    let promise = self
      .tsfn
      .call_async(Ok(payload))
      .await
      .map_err(|e| HandlerInvokeError::InvocationFailed(e.to_string()))?;

    let sleep = time::sleep(Duration::from_secs(self.timeout_secs));
    tokio::select! {
        _ = sleep => {
            Err(HandlerInvokeError::InvocationFailed("handler timed out".to_string()))
        }
        value = promise => {
            value.map_err(|e| HandlerInvokeError::InvocationFailed(e.to_string()))
        }
    }
  }
}
