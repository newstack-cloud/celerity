#!/bin/bash

cargo clippy -- -D warnings || exit 1
cargo fmt
