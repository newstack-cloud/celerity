use std::{fmt::Debug, future::Future, pin::Pin};

use serde_json::Value;

/// Provides a trait for a workflow state handler.
/// This is to be used to provide the functionality to carry out the work
/// of the `executeStep` workflow state type.
///
/// The handler can be a closure or a struct that implements this trait.
pub trait WorkflowStateHandler {
    /// The type of future calling this handler returns.
    type Future: Future<Output = Result<Value, WorkflowStateHandlerError>> + Send + 'static;

    /// Call the handler with the input data to the current workflow
    /// state and additional context.
    fn call(&self, input: Value) -> Self::Future;
}

impl Debug
    for dyn WorkflowStateHandler<
            Future = Pin<
                Box<dyn Future<Output = Result<Value, WorkflowStateHandlerError>> + Send + 'static>,
            >,
        > + Send
        + Sync
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "WorkflowStateHandler")
    }
}

impl<F, Fut> WorkflowStateHandler for F
where
    F: Fn(Value) -> Fut + Send + Sync + 'static,
    Fut: Future<Output = Result<Value, WorkflowStateHandlerError>> + Send + 'static,
{
    type Future = Pin<Box<dyn Future<Output = Result<Value, WorkflowStateHandlerError>> + Send>>;

    fn call(&self, value: Value) -> Self::Future {
        let fut = (self)(value);
        Box::pin(async move { fut.await })
    }
}

/// A boxed handler that can be used to store a workflow state
/// handler in a map.
pub type BoxedWorkflowStateHandler = Box<
    dyn WorkflowStateHandler<
            Future = Pin<Box<dyn Future<Output = Result<Value, WorkflowStateHandlerError>> + Send>>,
        > + Send
        + Sync,
>;

/// An error type used for workflow state handlers.
/// This must be the error type in the result yielded by workflow
/// state handler futures so that the state machine can handle
/// errors appropriately.
///
/// The `name` field is used as the error name in the `matchErrors` fields
/// in the retry and catch configuration of a workflow state.
pub struct WorkflowStateHandlerError {
    pub name: String,
    pub message: String,
}
