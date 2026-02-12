# Node.js runtime

The Node.js runtime is for applications where the handlers need to be written in Node.js.
This runtime interfaces with handlers through bi-directional FFI calls where the handler set up code calls into the runtime to register the handler functions and the runtime calls into the Node.js handler functions when an event, message or request is received.

The runtime uses the [Node.js SDK for Celerity](https://celerityframework.io/docs/node-runtime) to host developer application code that uses the core rust runtime to handle events, messages and requests under the hood.

## Additional documentation

- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)
