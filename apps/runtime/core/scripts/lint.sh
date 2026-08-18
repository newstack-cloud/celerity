#!/bin/bash

# `--all-targets` so the tests are linted too, which the pre-commit hook is the
# last chance to catch before CI runs the same check with `-D warnings`.
cargo clippy --all-targets -- -D warnings || exit 1
# `--check` rather than a rewrite, so that formatting drift fails the commit
# instead of being silently fixed in the working tree after the files staged for
# the commit have already been read.
cargo fmt --check || exit 1
