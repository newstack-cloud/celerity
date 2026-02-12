// Generates runtime-manifest.json listing installed @celerity-sdk/* versions.
// Run during Docker build: node generate-manifest.mjs

import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

function generateManifest() {
  const sdkDir = "./node_modules/@celerity-sdk";
  const manifest = { generatedAt: new Date().toISOString(), sdkVersions: {} };

  try {
    for (const pkg of readdirSync(sdkDir)) {
      try {
        const pkgJson = JSON.parse(
          readFileSync(join(sdkDir, pkg, "package.json"), "utf8"),
        );
        manifest.sdkVersions[pkg] = pkgJson.version;
      } catch {
        // Skip platform-specific binaries without a standard package.json
      }
    }
  } catch {
    // @celerity-sdk directory not found
  }

  writeFileSync("runtime-manifest.json", JSON.stringify(manifest, null, 2));
}

generateManifest();
