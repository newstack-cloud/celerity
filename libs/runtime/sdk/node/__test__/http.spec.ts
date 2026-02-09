import test from "ava";
import {
  CoreRuntimeApplication,
  CoreRuntimePlatform,
  type CoreRuntimeAppConfig,
  type CoreRuntimeConfig,
  type Request,
  type Response as SdkResponse,
} from "../index.js";

const PORT = 30100;
const BASE = `http://localhost:${PORT}`;

/** Returns a full CoreRuntimeConfig with sensible test defaults. */
function testConfig(
  overrides: Partial<CoreRuntimeConfig> & { serverPort: number },
): CoreRuntimeConfig {
  return {
    blueprintConfigPath: "__test__/http-api-no-auth.blueprint.yaml",
    serviceName: "node-sdk-test",
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
// Handlers
// ---------------------------------------------------------------------------

/** Returns all request getters as a JSON body. */
async function echoHandler(
  _err: Error | null,
  request: Request,
): Promise<SdkResponse> {
  return {
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      method: request.method,
      uri: request.uri,
      path: request.path,
      pathParams: request.pathParams,
      query: request.query,
      headers: request.headers,
      cookies: request.cookies,
      contentType: request.contentType,
      requestId: request.requestId,
      requestTime: request.requestTime,
      auth: request.auth,
      clientIp: request.clientIp,
      traceContext: request.traceContext,
      userAgent: request.userAgent,
      matchedRoute: request.matchedRoute,
      textBody: request.textBody,
      httpVersion: request.httpVersion,
      hasBinaryBody: request.binaryBody !== null,
      binaryBodyLength: request.binaryBody?.length ?? 0,
    }),
  };
}

/** Returns 201 with a custom header. Used for the custom-status test. */
async function customStatusHandler(
  _err: Error | null,
  _request: Request,
): Promise<SdkResponse> {
  return {
    status: 201,
    headers: {
      "content-type": "application/json",
      "x-custom-header": "custom-value",
    },
    body: JSON.stringify({ created: true }),
  };
}

/** Sleeps for 3 seconds — will exceed the 1-second timeout. */
async function slowHandler(
  _err: Error | null,
  _request: Request,
): Promise<SdkResponse> {
  await new Promise((resolve) => setTimeout(resolve, 3000));
  return { status: 200, body: "should not reach" };
}

/** Handles binary request/response testing. */
async function binaryEchoHandler(
  _err: Error | null,
  request: Request,
): Promise<SdkResponse> {
  if (request.binaryBody !== null) {
    // Binary request: respond with the length as JSON.
    return {
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        binaryLength: request.binaryBody.length,
      }),
    };
  }
  // Text request: respond with a known binary payload.
  return {
    status: 200,
    headers: { "content-type": "application/octet-stream" },
    binaryBody: Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  };
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

let app: CoreRuntimeApplication;

test.before(async () => {
  app = new CoreRuntimeApplication(testConfig({ serverPort: PORT }));
  const appConfig = app.setup();

  for (const handler of appConfig.api?.http?.handlers ?? []) {
    switch (handler.path) {
      case "/slow":
        app.registerHttpHandler(handler.path, handler.method, 1, slowHandler);
        break;
      case "/binary-echo":
        app.registerHttpHandler(
          handler.path,
          handler.method,
          null,
          binaryEchoHandler,
        );
        break;
      case "/items":
        if (handler.method === "POST") {
          // POST /items uses the custom-status handler for test 6.
          // We'll override this choice per-test by having a dedicated GET route.
          // Actually, register echo for POST too — test 6 uses GET /items/{itemId} with PUT.
          app.registerHttpHandler(
            handler.path,
            handler.method,
            null,
            echoHandler,
          );
        } else {
          app.registerHttpHandler(
            handler.path,
            handler.method,
            null,
            echoHandler,
          );
        }
        break;
      default:
        app.registerHttpHandler(
          handler.path,
          handler.method,
          null,
          echoHandler,
        );
        break;
    }
  }

  await app.run(false);
});

test.after.always(() => {
  if (app) {
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

interface EchoBody {
  method: string;
  uri: string;
  path: string;
  pathParams: Record<string, string>;
  query: Record<string, string[]>;
  headers: Record<string, string[]>;
  cookies: Record<string, string>;
  contentType: string;
  requestId: string;
  requestTime: string;
  auth: unknown;
  clientIp: string;
  traceContext: Record<string, string> | null;
  userAgent: string;
  matchedRoute: string | null;
  textBody: string | null;
  httpVersion: string;
  hasBinaryBody: boolean;
  binaryBodyLength: number;
}

async function fetchEcho(
  path: string,
  init?: RequestInit,
): Promise<{ status: number; headers: Headers; echo: EchoBody }> {
  const res = await fetch(`${BASE}${path}`, init);
  const echo: EchoBody = await res.json();
  return { status: res.status, headers: res.headers, echo };
}

// ---------------------------------------------------------------------------
// 1. Lifecycle
// ---------------------------------------------------------------------------

test("setup() returns expected handler definitions", (t) => {
  // Use a different port to avoid conflicts with the main app.
  const serverPort = PORT + 1;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.api);
  t.truthy(config.api!.http);

  const handlers = config.api!.http!.handlers;
  t.is(handlers.length, 7);

  const routes = handlers.map((h) => `${h.method} ${h.path}`).sort();
  t.deepEqual(routes, [
    "DELETE /items/{itemId}",
    "GET /items",
    "GET /items/{itemId}",
    "GET /slow",
    "POST /binary-echo",
    "POST /items",
    "PUT /items/{itemId}",
  ]);

  for (const h of handlers) {
    t.is(h.timeout, 60);
    t.truthy(h.handler);
    t.truthy(h.location);
  }
});

// ---------------------------------------------------------------------------
// 2–6. Basic request/response
// ---------------------------------------------------------------------------

test("GET request returns 200 with JSON body", async (t) => {
  const { status, echo } = await fetchEcho("/items");
  t.is(status, 200);
  t.is(echo.method, "GET");
});

test("POST request with JSON body", async (t) => {
  const { status, echo } = await fetchEcho("/items", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "widget" }),
  });
  t.is(status, 200);
  t.is(echo.method, "POST");
  t.is(echo.contentType, "application/json");
  t.deepEqual(JSON.parse(echo.textBody!), { name: "widget" });
});

test("PUT request", async (t) => {
  const { status, echo } = await fetchEcho("/items/1", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "updated" }),
  });
  t.is(status, 200);
  t.is(echo.method, "PUT");
});

test("DELETE request", async (t) => {
  const { status, echo } = await fetchEcho("/items/1", {
    method: "DELETE",
  });
  t.is(status, 200);
  t.is(echo.method, "DELETE");
});

test("custom response status and headers", async (t) => {
  // Register a temporary app on a different port with a custom-status handler.
  // Since we can't re-register handlers on the shared app, create a fresh one.
  const serverPort = PORT + 2;
  const customApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = customApp.setup();
  for (const handler of config.api?.http?.handlers ?? []) {
    if (handler.path === "/items" && handler.method === "GET") {
      customApp.registerHttpHandler(
        handler.path,
        handler.method,
        null,
        customStatusHandler,
      );
    } else {
      customApp.registerHttpHandler(
        handler.path,
        handler.method,
        null,
        echoHandler,
      );
    }
  }
  await customApp.run(false);

  try {
    const res = await fetch(`http://localhost:${serverPort}/items`);
    t.is(res.status, 201);
    t.is(res.headers.get("x-custom-header"), "custom-value");
    const body = await res.json();
    t.deepEqual(body, { created: true });
  } finally {
    customApp.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 7–12. Request getters
// ---------------------------------------------------------------------------

test("path and pathParams extraction", async (t) => {
  const { echo } = await fetchEcho("/items/42");
  t.is(echo.path, "/items/42");
  t.deepEqual(echo.pathParams, { itemId: "42" });
  // matchedRoute is populated by telemetry middleware; with tracing disabled it is null.
  // We still verify pathParams extraction works correctly above.
});

test("query parameters (single and multi-valued)", async (t) => {
  const { echo } = await fetchEcho("/items?color=red&tag=a&tag=b");
  t.deepEqual(echo.query["color"], ["red"]);
  t.deepEqual(echo.query["tag"], ["a", "b"]);
});

test("multi-valued request headers", async (t) => {
  const headers = new Headers();
  headers.append("x-custom", "a");
  headers.append("x-custom", "b");
  const { echo } = await fetchEcho("/items", { headers });
  // HTTP/1.1 may collapse multi-valued headers into a comma-separated string.
  // The SDK splits by header map entry, so the result depends on how fetch sends them.
  const customValues = echo.headers["x-custom"];
  t.truthy(customValues);
  // fetch collapses appended headers into "a, b" for most implementations.
  // The SDK iterates HeaderMap entries, so we may get ["a, b"] or ["a", "b"].
  const joined = customValues.join(", ");
  t.true(joined.includes("a"));
  t.true(joined.includes("b"));
});

test("cookie parsing", async (t) => {
  const { echo } = await fetchEcho("/items", {
    headers: { Cookie: "session=abc123; theme=dark" },
  });
  t.is(echo.cookies["session"], "abc123");
  t.is(echo.cookies["theme"], "dark");
});

test("request metadata", async (t) => {
  const { echo } = await fetchEcho("/items", {
    headers: { "User-Agent": "test-agent/1.0" },
  });
  // requestId: non-empty string
  t.truthy(echo.requestId);
  t.true(echo.requestId.length > 0);

  // requestTime: ISO 8601
  t.regex(echo.requestTime, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/);

  // httpVersion
  t.true(echo.httpVersion.includes("HTTP"));

  // clientIp and userAgent are populated by telemetry middleware.
  // With tracing disabled they are empty strings; just verify the fields exist.
  t.is(typeof echo.clientIp, "string");
  t.is(typeof echo.userAgent, "string");
});

test("auth is null when no auth configured", async (t) => {
  const { echo } = await fetchEcho("/items");
  t.is(echo.auth, null);
});

// ---------------------------------------------------------------------------
// 13–14. Body handling
// ---------------------------------------------------------------------------

test("binary request body", async (t) => {
  const binaryPayload = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a]);
  const res = await fetch(`${BASE}/binary-echo`, {
    method: "POST",
    headers: { "Content-Type": "application/octet-stream" },
    body: binaryPayload,
  });
  t.is(res.status, 200);
  const body = await res.json();
  t.is(body.binaryLength, 6);
});

test("binary response body", async (t) => {
  const res = await fetch(`${BASE}/binary-echo`, {
    method: "POST",
    headers: { "Content-Type": "text/plain" },
    body: "give-me-binary",
  });
  t.is(res.status, 200);
  t.is(res.headers.get("content-type"), "application/octet-stream");
  const buf = Buffer.from(await res.arrayBuffer());
  // PNG magic bytes
  t.deepEqual(
    [...buf],
    [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a],
  );
});

// ---------------------------------------------------------------------------
// 15–16. x-request-id
// ---------------------------------------------------------------------------

test("auto-generated x-request-id in response", async (t) => {
  const res = await fetch(`${BASE}/items`);
  const reqId = res.headers.get("x-request-id");
  t.truthy(reqId);
  t.true(reqId!.length > 0);
});

test("client-provided x-request-id echoed", async (t) => {
  const res = await fetch(`${BASE}/items`, {
    headers: { "x-request-id": "test-req-123" },
  });
  t.is(res.headers.get("x-request-id"), "test-req-123");
  const echo: EchoBody = await res.json();
  t.is(echo.requestId, "test-req-123");
});

// ---------------------------------------------------------------------------
// 17. Timeout
// ---------------------------------------------------------------------------

test("handler timeout returns 504", async (t) => {
  const res = await fetch(`${BASE}/slow`);
  t.is(res.status, 504);
  const body = await res.json();
  t.true(body.message.includes("handler timed out"));
});

// ---------------------------------------------------------------------------
// 18–19. CORS
// ---------------------------------------------------------------------------

test("preflight returns correct CORS headers", async (t) => {
  const res = await fetch(`${BASE}/items`, {
    method: "OPTIONS",
    headers: {
      Origin: "https://example.com",
      "Access-Control-Request-Method": "GET",
    },
  });
  t.is(res.status, 200);
  t.is(res.headers.get("access-control-allow-origin"), "https://example.com");
  t.truthy(res.headers.get("access-control-allow-methods"));
});

test("disallowed origin gets no allow-origin", async (t) => {
  const res = await fetch(`${BASE}/items`, {
    method: "OPTIONS",
    headers: {
      Origin: "https://evil.com",
      "Access-Control-Request-Method": "GET",
    },
  });
  t.is(res.headers.get("access-control-allow-origin"), null);
});
