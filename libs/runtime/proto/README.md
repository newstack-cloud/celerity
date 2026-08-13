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
