# Handler Invoke API

The Celerity Handler Invoke API allows developers to invoke handlers directly in their local development environments.
This is designed to be used in local development environments and is not intended for use in production. Handlers in the same application can invoke each other in production environments but use a separate, internal mechanism to do so.

This API is mostly useful for testing and debugging locally.

Where handlers run in a separate executable, an invocation is dispatched to that executable over the [IPC Handler Protocol](../../proto/README.md) rather than being run in the runtime process. It takes the same timeout, capacity and cancellation handling as any other event, so a caller can also see a `503` when the runtime cannot dispatch it at all and a `504` when the handler does not answer in time.

The runtime only serves this endpoint when it is explicitly enabled and the platform is local or test mode is on. It bypasses whatever normally triggers a handler and carries no authentication of its own, including when the API it shares a server with has a default auth guard configured.

[handler-invoke-api-v1](./handler-invoke-api-v1.yaml) - The Celerity Handler Invoke API v1 specification.
