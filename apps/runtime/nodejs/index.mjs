// Celerity Node.js Runtime Host — Entry Point
//
// Bridges the Rust NAPI runtime binary (@celerity-sdk/runtime) with developer
// application code via the @celerity-sdk/core framework.
//
// Prerequisites:
//   --import ./register-hooks.mjs   (SDK resolve hook — ensures single instance)
//   --import @celerity-sdk/telemetry/setup  (OTel auto-instrumentation)
//
// See: yarn start (production) / yarn dev (development with hot reload)

import process from "node:process";
import { startRuntime } from "@celerity-sdk/core";
import { checkSdkVersionAlignment, getRuntimeSdkVersions } from "./sdk-compat-check.mjs";

const LOG_PREFIX = "[celerity-runtime]";

const REQUIRED_ENV_VARS = [
  "CELERITY_BLUEPRINT",
  "CELERITY_RUNTIME_CALL_MODE",
  "CELERITY_SERVICE_NAME",
  "CELERITY_SERVER_PORT",
  "CELERITY_RUNTIME_PLATFORM",
  "CELERITY_MODULE_PATH",
];

async function main() {
  // 1. Validate required environment variables
  const missing = REQUIRED_ENV_VARS.filter((v) => !process.env[v]);
  if (missing.length > 0) {
    console.error(
      `${LOG_PREFIX} Missing required environment variables:\n` +
        missing.map((v) => `  - ${v}`).join("\n") +
        "\n\nSet these before starting the runtime. See .env.example for reference.",
    );
    process.exit(1);
  }

  // 2. SDK version alignment check (best-effort — never blocks startup)
  try {
    checkSdkVersionAlignment(import.meta.url);
  } catch {
    // intentionally ignored
  }

  // 3. Log startup info and SDK versions
  const sdkVersions = getRuntimeSdkVersions(import.meta.url);
  const sdkVersionStr = Object.entries(sdkVersions)
    .map(([pkg, ver]) => `${pkg}@${ver}`)
    .join(", ");

  const isDev = process.env.NODE_ENV !== "production";

  console.log(`${LOG_PREFIX} Starting Celerity Node.js runtime`);
  console.log(`${LOG_PREFIX} Service:  ${process.env.CELERITY_SERVICE_NAME}`);
  console.log(`${LOG_PREFIX} Port:     ${process.env.CELERITY_SERVER_PORT}`);
  console.log(`${LOG_PREFIX} Platform: ${process.env.CELERITY_RUNTIME_PLATFORM}`);
  console.log(`${LOG_PREFIX} Module:   ${process.env.CELERITY_MODULE_PATH}`);
  console.log(`${LOG_PREFIX} Mode:     ${isDev ? "development" : "production"}`);
  console.log(`${LOG_PREFIX} SDK:      ${sdkVersionStr}`);

  // 4. Graceful shutdown handling
  let shuttingDown = false;

  for (const signal of ["SIGTERM", "SIGINT"]) {
    process.on(signal, () => {
      if (shuttingDown) return;
      shuttingDown = true;
      console.log(`${LOG_PREFIX} Received ${signal}, shutting down...`);
      setTimeout(() => {
        console.error(`${LOG_PREFIX} Forced shutdown after timeout`);
        process.exit(1);
      }, 10_000).unref();
    });
  }

  // 5. Start the runtime
  await startRuntime({ block: true });
}

main().catch((error) => {
  console.error(`${LOG_PREFIX} Fatal error:`, error);
  process.exit(1);
});
