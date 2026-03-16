// ESM loader resolve hook — forces all @celerity-sdk/* imports to resolve
// from the runtime host's node_modules, ensuring a single shared instance
// of SDK packages regardless of what exists in the app's node_modules.
//
// Without this, duplicate Symbol-based metadata keys from separate copies
// of @celerity-sdk/core would cause the runtime to discover zero handlers.
//
// Resource packages (datastore, bucket, cache, queue, topic, sql-database)
// are app dependencies — they live in the app's node_modules, not the
// runtime's. When the runtime's core dynamically imports a resource package
// (e.g., in createDefaultSystemLayers), the original context.parentURL
// points to the runtime scope, so default resolution won't find them.
// The app anchor fallback handles this case.

import { pathToFileURL, fileURLToPath } from "node:url";
import { dirname, join, resolve as resolvePath } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const runtimeAnchorUrl = pathToFileURL(join(__dirname, "index.mjs")).href;

// Anchor URL for the app directory — used as a fallback when a @celerity-sdk/*
// package is not in the runtime's node_modules but exists in the app's.
const appDir = process.env.CELERITY_APP_DIR || join(__dirname, "app");
const appAnchorUrl = pathToFileURL(join(resolvePath(appDir), "index.mjs")).href;

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
      // Package not installed in the runtime's node_modules (e.g., resource
      // packages like @celerity-sdk/datastore). Try the app's node_modules
      // since the original context.parentURL may point to a runtime-hosted
      // package (e.g., @celerity-sdk/core) whose scope doesn't include the
      // app directory.
      try {
        return await nextResolve(specifier, {
          ...context,
          parentURL: appAnchorUrl,
        });
      } catch {
        // Not in the app's node_modules either — fall through to default
        // resolution with the original context as a last resort.
      }
    }
  }
  return nextResolve(specifier, context);
}
