// Registers the custom ESM resolve hook that ensures @celerity-sdk/* packages
// always resolve from the runtime host's node_modules.
//
// This must be the LAST --import flag before application imports so the SDK
// resolver is the outermost hook in the chain (Node.js calls last-registered
// hooks first). Place loader hooks like @swc-node/register BEFORE this flag.
//
// Usage: node --import ./register-hooks.mjs index.mjs

import { register } from "node:module";

register("./sdk-resolver.mjs", import.meta.url);
