import test from "ava";
import {
  CoreRuntimeApplication,
  CoreRuntimePlatform,
  type CoreRuntimeAppConfig,
  type CoreRuntimeConfig,
} from "../index.js";

const PORT = 30200;

function testConfig(
  overrides: Partial<CoreRuntimeConfig> & { serverPort: number },
): CoreRuntimeConfig {
  return {
    blueprintConfigPath: "__test__/custom-handler.blueprint.yaml",
    serviceName: "node-sdk-custom-handler-test",
    traceOtlpCollectorEndpoint: "",
    runtimeMaxDiagnosticsLevel: "info",
    platform: CoreRuntimePlatform.Local,
    testMode: true,
    resourceStoreVerifyTls: false,
    resourceStoreCacheEntryTtl: 600,
    resourceStoreCleanupInterval: 3600,
    serverLoopbackOnly: true,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// 1. Config test (no Valkey required)
// ---------------------------------------------------------------------------

test("setup() returns custom handler config", (t) => {
  const serverPort = PORT + 3;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.customHandlers);
  const handlers = config.customHandlers!.handlers;
  t.is(handlers.length, 1);
  t.is(handlers[0].name, "utilityHandler");
  t.is(handlers[0].timeout, 60);
});

// ---------------------------------------------------------------------------
// 2. Custom handler invocation via invoke API
// ---------------------------------------------------------------------------

test("custom handler invocation via invoke API", async (t) => {
  const serverPort = PORT + 13;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: "ok",
    }));
  }

  // Register the custom handler.
  for (const handler of config.customHandlers?.handlers ?? []) {
    app.registerCustomHandler(
      handler.name,
      handler.timeout,
      async (_err: Error | null, payload: unknown) => {
        return { received: payload, echoed: true };
      },
    );
  }

  await app.run(false);

  try {
    const res = await fetch(`http://localhost:${serverPort}/runtime/handlers/invoke`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        handlerName: "utilityHandler",
        invocationType: "requestResponse",
        payload: { input: "test-data" },
      }),
    });

    t.is(res.status, 200);
    const body = await res.json();
    t.is(body.message, "Handler invoked successfully");
    t.truthy(body.data);

    const data = JSON.parse(body.data);
    t.deepEqual(data.received, { input: "test-data" });
    t.is(data.echoed, true);
  } finally {
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 3. Custom handler timeout returns error
// ---------------------------------------------------------------------------

test("custom handler timeout returns error", async (t) => {
  const serverPort = PORT + 14;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: "ok",
    }));
  }

  // Register a slow custom handler with 1-second timeout.
  for (const handler of config.customHandlers?.handlers ?? []) {
    app.registerCustomHandler(handler.name, 1, async () => {
      await new Promise((r) => setTimeout(r, 5000));
      return { shouldNotReach: true };
    });
  }

  await app.run(false);

  try {
    const res = await fetch(`http://localhost:${serverPort}/runtime/handlers/invoke`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        handlerName: "utilityHandler",
        invocationType: "requestResponse",
        payload: {},
      }),
    });

    t.is(res.status, 500);
    const body = await res.json();
    t.truthy(body.message);
    t.true(body.message.includes("timed out"));
  } finally {
    app.shutdown();
  }
});
