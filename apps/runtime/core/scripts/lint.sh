#!/bin/bash

# `--all-targets` so the tests are linted too, which the pre-commit hook is the
# last chance to catch before CI runs the same check with `-D warnings`.
cargo clippy --all-targets -- -D warnings || exit 1
cargo fmt
