// Registers the custom ESM resolve hook that ensures @celerity-sdk/* packages
// always resolve from the runtime host's node_modules.
//
// This must be the FIRST --import flag so the hook is active before any other
// modules load (including @celerity-sdk/telemetry/setup and tsx).
//
// Usage: node --import ./register-hooks.mjs index.mjs

import { register } from "node:module";

register("./sdk-resolver.mjs", import.meta.url);
