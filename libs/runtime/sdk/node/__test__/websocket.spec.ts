import test from "ava";
import {
  CoreRuntimeApplication,
  CoreRuntimePlatform,
  JsMessageType,
  type CoreRuntimeAppConfig,
  type CoreRuntimeConfig,
  type JsWebSocketEventType,
  type JsWebSocketMessageInfo,
} from "../index.js";
import WebSocket from "ws";

const PORT = 30300;

function testConfig(
  overrides: Partial<CoreRuntimeConfig> & { serverPort: number },
): CoreRuntimeConfig {
  return {
    blueprintConfigPath: "__test__/ws-only.blueprint.yaml",
    serviceName: "node-sdk-ws-test",
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
// 1. Config tests
// ---------------------------------------------------------------------------

test("setup() returns websocket handler config", (t) => {
  const serverPort = PORT + 1;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.api);
  t.truthy(config.api!.websocket);

  const handlers = config.api!.websocket!.handlers;
  t.true(handlers.length >= 4);

  const routes = handlers.map((h) => h.route).sort();
  t.deepEqual(routes, ["$connect", "$default", "$disconnect", "echo"]);
});

// ---------------------------------------------------------------------------
// 2. WebSocket connect triggers $connect handler
// ---------------------------------------------------------------------------

test("WS connect triggers $connect handler", async (t) => {
  const serverPort = PORT + 10;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  const connectReceived = deferred<JsWebSocketMessageInfo>();

  // Register WS handlers.
  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "$connect":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            connectReceived.resolve(msg);
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    try {
      const timer = setTimeout(
        () => connectReceived.reject(new Error("timeout")),
        5000,
      );
      const msg = await connectReceived.promise;
      clearTimeout(timer);

      t.is(msg.eventType, "connect" as JsWebSocketEventType);
      t.truthy(msg.connectionId);
    } finally {
      ws.close();
    }
  } finally {
    // Allow disconnect to propagate.
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 3. WS JSON message routes to named handler
// ---------------------------------------------------------------------------

test("WS JSON message routes to echo handler", async (t) => {
  const serverPort = PORT + 11;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();


  const echoReceived = deferred<JsWebSocketMessageInfo>();

  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "echo":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            echoReceived.resolve(msg);
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    try {
      // Send a JSON message with the "echo" action route key.
      ws.send(JSON.stringify({ action: "echo", data: "hello" }));

      const timer = setTimeout(
        () => echoReceived.reject(new Error("timeout")),
        5000,
      );
      const msg = await echoReceived.promise;
      clearTimeout(timer);

      t.is(msg.messageType, "json");
      t.is(msg.eventType, "message" as JsWebSocketEventType);
      t.truthy(msg.connectionId);
      t.truthy(msg.jsonBody);
      t.is(msg.jsonBody!.data, "hello");
    } finally {
      ws.close();
    }
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 4. WS $default route for unmatched messages
// ---------------------------------------------------------------------------

test("WS $default route handles unmatched action", async (t) => {
  const serverPort = PORT + 12;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();


  const defaultReceived = deferred<JsWebSocketMessageInfo>();

  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "$default":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            defaultReceived.resolve(msg);
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    try {
      // Send a message with an unknown action — should route to $default.
      ws.send(JSON.stringify({ action: "unknown-action", data: "test" }));

      const timer = setTimeout(
        () => defaultReceived.reject(new Error("timeout")),
        5000,
      );
      const msg = await defaultReceived.promise;
      clearTimeout(timer);

      t.is(msg.eventType, "message" as JsWebSocketEventType);
      t.truthy(msg.jsonBody);
      t.is(msg.jsonBody!.action, "unknown-action");
    } finally {
      ws.close();
    }
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 5. WS disconnect triggers $disconnect handler
// ---------------------------------------------------------------------------

test("WS disconnect triggers $disconnect handler", async (t) => {
  const serverPort = PORT + 13;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();


  const disconnectReceived = deferred<JsWebSocketMessageInfo>();

  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "$disconnect":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            disconnectReceived.resolve(msg);
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    // Close the connection to trigger $disconnect.
    ws.close();

    const timer = setTimeout(
      () => disconnectReceived.reject(new Error("timeout")),
      5000,
    );
    const msg = await disconnectReceived.promise;
    clearTimeout(timer);

    t.is(msg.eventType, "disconnect" as JsWebSocketEventType);
    t.truthy(msg.connectionId);
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 6. WebSocket registry sendMessage
// ---------------------------------------------------------------------------

test("websocketRegistry sendMessage sends to connected client", async (t) => {
  const serverPort = PORT + 14;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();


  const registry = app.websocketRegistry();
  let capturedConnectionId = "";
  const connectDone = deferred<void>();

  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "$connect":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            capturedConnectionId = msg.connectionId;
            connectDone.resolve();
          },
        );
        break;
      case "echo":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
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
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = await openWs(`ws://localhost:${serverPort}/ws`);
    try {
      // Wait for connect handler to capture the connection ID.
      const connectTimer = setTimeout(
        () => connectDone.reject(new Error("timeout waiting for connect")),
        5000,
      );
      await connectDone.promise;
      clearTimeout(connectTimer);

      t.truthy(capturedConnectionId);

      // Send a message that triggers the echo handler.
      ws.send(JSON.stringify({ action: "echo", data: "ping" }));

      // Wait for the echoed response.
      const response = await nextWsMessage(ws);
      const parsed = JSON.parse(response);
      t.truthy(parsed.echo);
      t.is(parsed.echo.data, "ping");
    } finally {
      ws.close();
    }
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 7. WS request context includes headers/query
// ---------------------------------------------------------------------------

test("WS request context includes connection metadata", async (t) => {
  const serverPort = PORT + 15;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();


  const connectReceived = deferred<JsWebSocketMessageInfo>();

  for (const handler of config.api?.websocket?.handlers ?? []) {
    switch (handler.route) {
      case "$connect":
        app.registerWebsocketHandler(
          handler.route,
          async (_err: Error | null, msg: JsWebSocketMessageInfo) => {
            connectReceived.resolve(msg);
          },
        );
        break;
      default:
        app.registerWebsocketHandler(handler.route, async (_err, _msg) => { });
        break;
    }
  }

  await app.run(false);

  try {
    const ws = new WebSocket(`ws://localhost:${serverPort}/ws?token=abc123`, {
      headers: { "x-custom-header": "test-value", origin: "https://example.com" },
    });

    await new Promise<void>((resolve, reject) => {
      ws.on("open", () => resolve());
      ws.on("error", reject);
    });

    try {
      const timer = setTimeout(
        () => connectReceived.reject(new Error("timeout")),
        5000,
      );
      const msg = await connectReceived.promise;
      clearTimeout(timer);

      t.truthy(msg.requestContext);
      const ctx = msg.requestContext!;
      t.truthy(ctx.requestId);
      t.truthy(ctx.path);
      t.truthy(ctx.clientIp);
    } finally {
      ws.close();
    }
  } finally {
    await new Promise((r) => setTimeout(r, 200));
    app.shutdown();
  }
});
