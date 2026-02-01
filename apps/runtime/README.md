# runtime

Here, you'll find a collection of runtime implementations that can be used to run your Celerity applications. 

The runtime is designed to take the same inputs that the Celerity CLI takes to deploy serverless applications and run them locally or in a deployed environment via container or VM orchestration.
One of the key ideas behind Celerity is to remove the need to build applications differently depending on the target environment. This means that you can develop and test your applications locally, and then deploy them to a serverless or containerised environment without having to make any changes to the application code.

The runtime is responsible for processing incoming requests, messages/events, and routing them to the appropriate handlers defined by developers.

## [node.js runtime](./nodejs/README.md)

The Node.js runtime is for applications where the handlers need to be written in JavaScript or TypeScript.
This runtime interfaces with handlers through in-process FFI calls through bindings wrapped in the Celerity Node SDK.

Developers should make use of the Celerity Node SDK for a smoother development experience where interactions with the runtime are taken care of and a useful plugin/middleware system provides standard functionality such as loading secrets and dependency injection.


## [python runtime](./python/README.md)

The Python runtime is for applications where the handlers need to be written in Python.
This runtime interfaces with handlers through in-process FFI calls through bindings wrapped in the Celerity Python SDK.

Developers should make use of the Celerity Python SDK for a smoother development experience where interactions with the runtime are taken care of and a useful plugin/middleware system provides standard functionality such as loading secrets and dependency injection.

## [core runtime](./core/README.md)

**_Coming soon as a part of Celerity v1_**

The core (os-only) runtime is for applications where the handlers need to be written in a language that is compiled ahead of time, such as Rust, C, C++ or Go.
This runtime interfaces with handlers by exposing a HTTP API.

Developers can make use of the Celerity SDKs for Rust and Go for a smoother development experience where interactions with the runtime are taken care of and a useful plugin/middleware system provides standard functionality such as loading secrets and dependency injection.

## [c# runtime](./csharp/README.md)

**_Coming soon as a part of Celerity v1_**

The C# runtime is for applications where the handlers need to be written in C#.
This runtime interfaces with handlers through in-process FFI calls through bindings wrapped in the Celerity C# SDK.

Developers should make use of the Celerity C# SDK for a smoother development experience where interactions with the runtime are taken care of and a useful plugin/middleware system provides standard functionality such as loading secrets and dependency injection.

## [java runtime](./java/README.md)

**_Coming soon as a part of Celerity v1_**

The Java runtime is for applications where the handlers need to be written in Java.
This runtime interfaces with handlers through in-process FFI calls through bindings wrapped in the Celerity Java SDK.

Developers should make use of the Celerity Java SDK for a smoother development experience where interactions with the runtime are taken care of and a useful plugin/middleware system provides standard functionality such as loading secrets and dependency injection.

# Additional docs

- [HTTP API Docs](../../libs/runtime/api-docs/README.md) - API docs for the Local Runtime and Handler Invoke APIs where the former is used for handler <-> runtime communication for the os-only runtime.
