# Generates runtime-manifest.json listing installed celerity SDK versions.
# Run during Docker build: python generate_manifest.py

from __future__ import annotations

import contextlib
import json
from datetime import UTC, datetime
from importlib.metadata import PackageNotFoundError, version

SDK_PACKAGES = ["celerity-sdk", "celerity-runtime-sdk"]


def generate_manifest() -> None:
    sdk_versions: dict[str, str] = {}
    for pkg in SDK_PACKAGES:
        with contextlib.suppress(PackageNotFoundError):
            sdk_versions[pkg] = version(pkg)

    manifest = {
        "generatedAt": datetime.now(UTC).isoformat(),
        "sdkVersions": sdk_versions,
    }

    with open("runtime-manifest.json", "w") as f:
        json.dump(manifest, f, indent=2)


if __name__ == "__main__":
    generate_manifest()
