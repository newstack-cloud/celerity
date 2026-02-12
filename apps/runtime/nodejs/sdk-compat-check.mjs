// SDK version alignment check — compares the app's declared @celerity-sdk/*
// dependency ranges against the runtime's installed versions.
// Warns on mismatch but never blocks startup.

import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";

const LOG_PREFIX = "[celerity-runtime]";

/**
 * Simple semver range check for ^x.y.z and ~x.y.z patterns.
 * Compares major.minor without requiring a semver dependency.
 */
function satisfiesRange(installed, range) {
  const baseMatch = range.match(/(\d+)\.(\d+)\.(\d+)/);
  const instMatch = installed.match(/(\d+)\.(\d+)\.(\d+)/);
  if (!baseMatch || !instMatch) return true; // Can't parse, assume OK

  const baseMajor = Number(baseMatch[1]);
  const baseMinor = Number(baseMatch[2]);
  const basePatch = Number(baseMatch[3]);
  const instMajor = Number(instMatch[1]);
  const instMinor = Number(instMatch[2]);
  const instPatch = Number(instMatch[3]);

  if (range.startsWith("^")) {
    // ^x.y.z: same major, installed >= base
    return (
      instMajor === baseMajor &&
      (instMinor > baseMinor || (instMinor === baseMinor && instPatch >= basePatch))
    );
  }

  if (range.startsWith("~")) {
    // ~x.y.z: same major.minor, installed patch >= base patch
    return instMajor === baseMajor && instMinor === baseMinor && instPatch >= basePatch;
  }

  // Exact or other — just check major.minor match
  return instMajor === baseMajor && instMinor === baseMinor;
}

/**
 * Locate the app's package.json by walking up from CELERITY_MODULE_PATH,
 * falling back to CELERITY_APP_DIR.
 */
function findAppPackageJson() {
  const appDir = process.env.CELERITY_APP_DIR || "./app";
  const modulePath = process.env.CELERITY_MODULE_PATH;

  const searchDirs = [];
  if (modulePath) {
    let dir = join(resolve(modulePath), "..");
    for (let i = 0; i < 5; i++) {
      searchDirs.push(dir);
      dir = join(dir, "..");
    }
  }
  searchDirs.push(resolve(appDir));

  for (const dir of searchDirs) {
    try {
      return JSON.parse(readFileSync(join(dir, "package.json"), "utf8"));
    } catch {
      // Not found in this directory, try next
    }
  }
  return null;
}

/**
 * Read installed @celerity-sdk/* versions from the runtime's node_modules.
 * Returns a map of package short name to version string.
 *
 * @param {URL | string} runtimeBaseUrl - import.meta.url of the calling module
 */
export function getRuntimeSdkVersions(runtimeBaseUrl) {
  const sdkDir = new URL("./node_modules/@celerity-sdk", runtimeBaseUrl).pathname;
  const versions = {};
  try {
    for (const pkg of readdirSync(sdkDir)) {
      try {
        const pkgJson = JSON.parse(
          readFileSync(join(sdkDir, pkg, "package.json"), "utf8"),
        );
        versions[pkg] = pkgJson.version;
      } catch {
        // Skip platform-specific binaries without a standard package.json
      }
    }
  } catch {
    // @celerity-sdk directory not found
  }
  return versions;
}

/**
 * Compare the app's declared @celerity-sdk/* dependency ranges against
 * the runtime's installed versions. Logs warnings on mismatch.
 *
 * @param {URL | string} runtimeBaseUrl - import.meta.url of the calling module
 */
export function checkSdkVersionAlignment(runtimeBaseUrl) {
  const appPkg = findAppPackageJson();
  if (!appPkg) return;

  const appDeps = {
    ...appPkg.dependencies,
    ...appPkg.peerDependencies,
    ...appPkg.devDependencies,
  };

  const runtimeNodeModules = new URL("./node_modules", runtimeBaseUrl).pathname;
  const mismatches = [];

  for (const [name, range] of Object.entries(appDeps)) {
    if (!name.startsWith("@celerity-sdk/") || typeof range !== "string") continue;

    try {
      const installedPkg = JSON.parse(
        readFileSync(join(runtimeNodeModules, name, "package.json"), "utf8"),
      );
      if (!satisfiesRange(installedPkg.version, range)) {
        mismatches.push({ name, declared: range, installed: installedPkg.version });
      }
    } catch {
      // Package not in runtime's node_modules (e.g., @celerity-sdk/bucket) — skip
    }
  }

  if (mismatches.length > 0) {
    console.warn(`${LOG_PREFIX} SDK version mismatch detected:`);
    for (const m of mismatches) {
      console.warn(`  ${m.name}: app declares ${m.declared}, runtime provides ${m.installed}`);
    }
    console.warn(
      `${LOG_PREFIX} Consider aligning your app's SDK versions with the runtime.`,
    );
  }
}
