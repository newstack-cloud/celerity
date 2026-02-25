import test from "ava";
import {
  CoreRuntimeApplication,
  CoreRuntimePlatform,
  JsMessageType,
  type CoreRuntimeAppConfig,
  type CoreRuntimeConfig,
  type JsWebSocketMessageInfo,
} from "../index.js";
import WebSocket from "ws";

const PORT = 30400;

function testConfig(
  overrides: Partial<CoreRuntimeConfig> & { serverPort: number },
): CoreRuntimeConfig {
  return {
    blueprintConfigPath: "__test__/ws-http-hybrid.blueprint.yaml",
    serviceName: "node-sdk-ws-hybrid-test",
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

/** Creates a deferred promise for coordinating async handler callbacks. */
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** Opens a WebSocket and waits for the connection to be established. */
function openWs(url: string): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url, {
      headers: { origin: "https://example.com" },
    });
    ws.on("open", () => resolve(ws));
    ws.on("error", reject);
  });
}

/** Waits for the next message on a WebSocket. */
function nextWsMessage(ws: WebSocket, timeoutMs = 5000): Promise<string> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () => reject(new Error("timeout waiting for WS message")),
      timeoutMs,
    );
    ws.once("message", (data) => {
      clearTimeout(timer);
      resolve(data.toString());
    });
  });
}

// ---------------------------------------------------------------------------
// 1. Config: setup() returns both HTTP and WebSocket handler configs
// ---------------------------------------------------------------------------

test("setup() returns both HTTP and WebSocket handler configs", (t) => {
  const serverPort = PORT + 1;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.api);

  // HTTP handlers should be present.
  t.truthy(config.api!.http);
  t.true(config.api!.http!.handlers.length >= 1);

  // WebSocket handlers should be present.
  t.truthy(config.api!.websocket);
  const wsRoutes = config.api!.websocket!.handlers.map((h) => h.route).sort();
  t.deepEqual(wsRoutes, ["$connect", "$default", "$disconnect", "echo"]);
});

// ---------------------------------------------------------------------------
// 2. HTTP and WS coexist: both endpoints reachable on same server
// ---------------------------------------------------------------------------

test("HTTP and WS endpoints coexist on the same server", async (t) => {
  const serverPort = PORT + 10;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  // Register HTTP handlers.
  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: JSON.stringify({ status: "ok" }),
      headers: { "content-type": "application/json" },
    }));
  }

  const registry = app.websocketRegistry();
  const echoReceived = deferred<JsWebSocketMessageInfo>();

  // Register WS handlers.
  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "echo":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            echoReceived.resolve(msg);
            // Echo back via the registry.
            await registry.sendMessage(
              msg.connectionId,
              msg.messageId,
              "json" as JsMessageType,
              JSON.stringify({ echo: msg.jsonBody }),
              null,
            );
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => {});
        break;
    }
  }

  await app.run(false);

  try {
    // Verify HTTP endpoint works.
    const httpRes = await fetch(`http://localhost:${serverPort}/health`);
    t.is(httpRes.status, 200);

    // Verify WebSocket works on the same server.
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    try {
      ws.send(JSON.stringify({ action: "echo", data: "hybrid-test" }));

      const timer = setTimeout(
        () => echoReceived.reject(new Error("timeout")),
        5000,
      );
      await echoReceived.promise;
      clearTimeout(timer);

      const response = await nextWsMessage(ws);
      const parsed = JSON.parse(response);
      t.truthy(parsed.echo);
      t.is(parsed.echo.data, "hybrid-test");
    } finally {
      ws.close();
    }
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});
