# IPC Handler Protocol

The contract between the Celerity runtime and a handlers executable running as a
separate process, used by SDKs for ahead-of-time compiled languages such as Go
and Rust.

`celerity/runtime/v1/runtime.proto` is the single source of truth. Everything
else, in this repository and in SDK repositories, is generated from or written
against it.

## The generated Rust is checked in

The Rust stubs live at
[`../core/src/generated/celerity.runtime.v1.rs`](../core/src/generated/celerity.runtime.v1.rs)
and are committed, rather than produced by a build script.

Building the runtime therefore needs nothing beyond a Rust toolchain. The SDK release workflows cross-compile inside containers and
virtual machines, so a build-time compiler would have to be present in every
matrix leg of every one of them, and a leg that was missed would fail at release
time rather than in CI.

The trade-off is that regeneration is a manual step. **Changing the `.proto` without
regenerating leaves the two out of step**, so do both in the same commit. CI
regenerates and fails on any difference, so this is caught rather than merged.

## Regenerating after changing the `.proto`

Install [`buf`](https://buf.build/docs/installation):

```bash
# macOS
brew install bufbuild/buf/buf

# or download a release directly
# https://github.com/bufbuild/buf/releases
```

Then, from `libs/runtime`:

```bash
cargo run -p celerity-proto-gen
```

That rewrites the generated module in place. Commit the `.proto` and the
regenerated Rust together, and review the generated diff as part of the change:
it is the clearest signal of whether an edit to the contract was additive or
breaking.

`buf` compiles the contract and the generator turns the resulting descriptor set
into Rust, so `buf` is the only tool the protocol needs, the same one that lints
it and checks it for compatibility. Nothing here uses `protoc` directly, which
means CI and a developer's machine cannot disagree about which compiler produced
the checked-in stubs.

## Compatibility

The package is `celerity.runtime.v1`. Adding fields and messages is backwards
compatible and expected. Renaming or renumbering an existing field, changing its
type, or removing it is not, and requires a `v2` package served alongside `v1`
through a deprecation window.

`buf.yaml` and `buf.gen.yaml` are present so that `buf lint` and
`buf breaking --against` can be run against the contract, and so that stubs for
SDKs in other languages can be generated from the same configuration. Neither is
wired into CI yet; that follows once the contract stops moving.

## What is typed and what is not

Every field the runtime needs in order to make a decision is typed. User
payloads are `bytes`, which the runtime moves without inspecting.

This is not an aesthetic choice. The runtime currently deserialises and
re-serialises user data it never looks at, which costs 356µs on a 1,000-field
payload against 3.0µs for passthrough. It is also why `google.protobuf.Struct`
does not appear anywhere in the event path: its cost scales with field count
rather than bytes, making it slower than the JSON in production today, larger on
the wire, and lossy for integers, byte strings and non-finite numbers.

## What an SDK on the handler side must do

The `.proto` is the authority on message shapes, field numbers and wire
compatibility. What it cannot express is ordering, timing and obligation, and
those are what an SDK author gets wrong. This section is the short form; the
normative specification is the
[Handler IPC Protocol](https://celerityframework.io/docs/framework/runtime/handler-ipc-protocol)
document.

An implementation must:

1. **Connect** to `CELERITY_RUNTIME_SOCKET`, inherited from the runtime that
   started the process, and open exactly one `EventStream`.
2. **Accept `RuntimeConfig` as the first frame.** It arrives before anything is
   asked, so an SDK can check the handlers registered in code against the ones
   the blueprint declares, and refuse for itself rather than waiting to be
   refused.
3. **Send `Ready` within 30 seconds**, declaring the protocol version it was
   built against, every tag it serves, and an initial credit no larger than its
   worker pool. The timeout is not configurable.
4. **Stop, reporting why, where `ReadyAck` is not accepted.** Refusal is a tag
   mismatch in either direction, or a protocol major version the runtime does
   not serve.
5. **Answer every `Dispatch` with a `Result` carrying the same id** — including
   where user code panicked, threw, or was cancelled.
6. **Send a `credit_grant` on every result, including on failure.** Credit is
   consumed on dispatch and replenished only when the handler says so. A missed
   grant drains the window and stalls the stream silently, which is the single
   easiest way to write an SDK that works under test and hangs under load. Zero
   is for withholding deliberately.
7. **Report failure for a message it did not process**, naming records by the
   id they arrived with. For a source that acknowledges, the outcome decides
   whether the message is delivered again.
8. **Treat `deadline_unix_ms` as its own deadline.** The runtime enforces it
   independently.
9. **Ignore a `Cancel` for an event it no longer holds.** Cancellation is
   advisory and races with completion; a cancelled event should still be
   answered, because that is what returns the credit.
10. **Stop taking new work on `Drain`** and finish what it holds before the
    deadline.
11. **Answer nothing after the stream closes.**

### Handler tags

A tag is how the runtime addresses a handler, and it is compared byte for byte
at the handshake. The constructions are in
`libs/runtime/core/src/event_queue.rs`:

```
HTTP       <METHOD>::<route>          e.g. GET::/orders/{orderId}
WebSocket  <routeKey>::<route>
Consumer   source::<sourceId>::<handlerName>
Schedule   source::<sourceId>::<handlerName>
Custom     custom::<handlerName>
```

An SDK does not have to build these. `RuntimeConfig.handlers[].handler_tag`
carries the tag the runtime will address each handler by, so echoing those back
in `Ready.handler_tags` is both simpler and impossible to get wrong by a byte.
Reconstructing them is what the official SDKs do, and it is why their tests pin
the format.

Two details that catch people out: a blueprint's `/files/{path+}` reaches the
handler as `/files/{*path}`, the router's form and the form the tag is built
from; and a binary WebSocket message must be framed in the Celerity Binary
Message Format or the runtime refuses it.

### Version negotiation

`ProtocolVersion` is carried both ways. The runtime serves major version `1`.
A handler declaring no version is refused, as is one declaring a different
major — at the handshake, rather than later on a frame it cannot read. Minor
versions are additive, so a handler built against an earlier minor of the same
major serves here unchanged.

### A worked reference

`libs/runtime/core/tests/common/ipc.rs` is a minimal working handler client —
the connect idiom, the config-then-`Ready`-then-`ReadyAck` handshake, the frame
loop, and a credit grant on every result. It is the shortest complete thing to
read before writing an SDK.
