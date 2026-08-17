# celerity runtime helpers

This crate holds the pieces more than one Celerity runtime component needs, so
that they are written once and agree with each other.

The most prominent of these being the WebSocket message format, where a parser and an encoder
sit together. A message read differently from the way it was written is a
message nobody receives, so the two belong side by side rather than in the
crates that happen to use them.

The rest is the ordinary shared ground of a runtime: reading configuration from
the environment, request and response helpers, telemetry setup for tracing and
metrics, retry and backoff, time, JSONPath, and the client pieces for consumers
and Redis.

## Additional documentation

- [Contributing](../CONTRIBUTING.md)
