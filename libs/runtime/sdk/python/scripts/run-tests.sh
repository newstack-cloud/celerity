#!/bin/bash

if [ -n "$GITHUB_ACTIONS" ]; then
  # Run directly in python environment when in CI job.
  maturin build
  pip install .
  cp -r build/lib*/celerity_runtime_sdk/ celerity_runtime_sdk/
  python -m pytest tests/
else
  # Run with pipenv in local envs.
  pipenv run maturin develop
  pipenv run python -m pytest tests/
fi
