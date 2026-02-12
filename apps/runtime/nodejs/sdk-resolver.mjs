// ESM loader resolve hook — forces all @celerity-sdk/* imports to resolve
// from the runtime host's node_modules, ensuring a single shared instance
// of SDK packages regardless of what exists in the app's node_modules.
//
// Without this, duplicate Symbol-based metadata keys from separate copies
// of @celerity-sdk/core would cause the runtime to discover zero handlers.

import { pathToFileURL, fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const runtimeAnchorUrl = pathToFileURL(join(__dirname, "index.mjs")).href;

export async function resolve(specifier, context, nextResolve) {
  if (specifier.startsWith("@celerity-sdk/")) {
    try {
      // Resolve from the runtime host's node_modules by overriding parentURL.
      // This makes Node's default resolver look in /opt/celerity/node_modules/
      // (or wherever the runtime host lives) instead of the app's node_modules/.
      // Correctly handles subpath exports (e.g., @celerity-sdk/telemetry/setup).
      return await nextResolve(specifier, {
        ...context,
        parentURL: runtimeAnchorUrl,
      });
    } catch {
      // Package not installed in the runtime's node_modules
      // (e.g., @celerity-sdk/bucket — a cloud service package the app uses directly).
      // Fall through to default resolution so the app's own copy is used.
      // This is safe because cloud service packages don't share Symbol-keyed
      // state with the runtime core.
    }
  }
  return nextResolve(specifier, context);
}
