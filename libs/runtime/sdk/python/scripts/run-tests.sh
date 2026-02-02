#!/bin/bash

uv run maturin develop
uv run python -m pytest -rA tests/
