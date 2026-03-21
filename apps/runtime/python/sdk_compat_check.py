# SDK version alignment check — compares the app's declared celerity-sdk
# dependency ranges against the runtime's installed versions.
# Warns on mismatch but never blocks startup.

from __future__ import annotations

import contextlib
import os
import re
import tomllib
from importlib.metadata import PackageNotFoundError, version
from pathlib import Path
from typing import Any

LOG_PREFIX = "[celerity-runtime]"

SDK_PACKAGES = ["celerity-sdk", "celerity-runtime-sdk"]


def get_runtime_sdk_versions() -> dict[str, str]:
    """Return installed versions of celerity SDK packages."""
    versions: dict[str, str] = {}
    for pkg in SDK_PACKAGES:
        with contextlib.suppress(PackageNotFoundError):
            versions[pkg] = version(pkg)
    return versions


def _find_app_pyproject() -> dict[str, Any] | None:
    """Locate the app's pyproject.toml by searching from CELERITY_MODULE_PATH."""

    app_dir = os.environ.get("CELERITY_APP_DIR", "./app")
    module_path = os.environ.get("CELERITY_MODULE_PATH")

    search_dirs: list[Path] = []
    if module_path:
        d = Path(module_path).resolve().parent
        for _ in range(5):
            search_dirs.append(d)
            d = d.parent
    search_dirs.append(Path(app_dir).resolve())

    for d in search_dirs:
        pyproject = d / "pyproject.toml"
        if pyproject.is_file():
            with open(pyproject, "rb") as f:
                return tomllib.load(f)
    return None


def _parse_version_from_spec(spec: str) -> tuple[str, str] | None:
    """Extract operator and version from a PEP 440 specifier like '>=0.2.0'."""
    match = re.match(r"([><=!~]+)\s*(\d+\.\d+\.\d+)", spec)
    if match:
        return match.group(1), match.group(2)
    return None


def _satisfies_range(installed: str, spec: str) -> bool:
    """Simple version range check for >=x.y.z patterns."""
    parsed = _parse_version_from_spec(spec)
    if not parsed:
        return True  # Can't parse, assume OK

    op, base = parsed
    inst_parts = [int(x) for x in installed.split(".")[:3]]
    base_parts = [int(x) for x in base.split(".")[:3]]

    if op == ">=":
        return inst_parts >= base_parts
    if op in ("==", "==="):
        return inst_parts == base_parts
    if op == "~=":
        # Compatible release: same major.minor, patch >= base
        return inst_parts[:2] == base_parts[:2] and inst_parts[2] >= base_parts[2]
    # Other operators — assume OK
    return True


def check_sdk_version_alignment() -> None:
    """Compare the app's declared SDK deps against installed versions."""
    pyproject = _find_app_pyproject()
    if not pyproject:
        return

    deps: list[str] = []
    project = pyproject.get("project", {})
    deps.extend(project.get("dependencies", []))
    for extras in project.get("optional-dependencies", {}).values():
        deps.extend(extras)

    installed = get_runtime_sdk_versions()
    mismatches: list[tuple[str, str, str]] = []

    for dep in deps:
        for pkg_name in SDK_PACKAGES:
            if not dep.startswith(pkg_name):
                continue
            # Strip extras like [runtime]
            spec_part = re.sub(r"\[.*?\]", "", dep)
            # Extract version specifier
            spec_match = re.search(r"([><=!~]+\s*\d+\.\d+\.\d+)", spec_part)
            if not spec_match or pkg_name not in installed:
                continue
            spec = spec_match.group(1)
            if not _satisfies_range(installed[pkg_name], spec):
                mismatches.append((pkg_name, spec, installed[pkg_name]))

    if mismatches:
        print(f"{LOG_PREFIX} SDK version mismatch detected:")
        for name, declared, inst in mismatches:
            print(f"  {name}: app declares {declared}, runtime provides {inst}")
        print(f"{LOG_PREFIX} Consider aligning your app's SDK versions with the runtime.")
