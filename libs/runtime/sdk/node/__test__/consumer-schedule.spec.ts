import test from "ava";
import {
  CoreRuntimeApplication,
  CoreRuntimePlatform,
  type CoreRuntimeAppConfig,
  type CoreRuntimeConfig,
  type JsConsumerEventInput,
  type JsScheduleEventInput,
  type JsEventResult,
} from "../index.js";
import Redis from "ioredis";

const PORT = 30200;
const REDIS_URL = process.env.CELERITY_LOCAL_REDIS_URL ?? "redis://127.0.0.1:6379";

function testConfig(
  overrides: Partial<CoreRuntimeConfig> & { serverPort: number },
): CoreRuntimeConfig {
  return {
    blueprintConfigPath: "__test__/consumer-schedule.blueprint.yaml",
    serviceName: "node-sdk-consumer-test",
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

/** Parses a redis:// URL into {host, port}. */
function parseRedisUrl(url: string): { host: string; port: number } {
  const u = new URL(url);
  return { host: u.hostname, port: Number(u.port) || 6379 };
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

// ---------------------------------------------------------------------------
// 1. Config tests (no Valkey required)
// ---------------------------------------------------------------------------

test("setup() returns consumer config", (t) => {
  const serverPort = PORT + 1;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.consumers);
  const consumers = config.consumers!.consumers;
  t.is(consumers.length, 1);
  t.is(consumers[0].sourceId, "test-order-queue");
  t.is(consumers[0].batchSize, 10);
  t.truthy(consumers[0].handlers.length > 0);
  t.is(consumers[0].handlers[0].name, "orderHandler");
});

test("setup() returns schedule config", (t) => {
  const serverPort = PORT + 2;
  const testApp = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config: CoreRuntimeAppConfig = testApp.setup();

  t.truthy(config.schedules);
  const schedules = config.schedules!.schedules;
  t.is(schedules.length, 1);
  t.is(schedules[0].scheduleValue, "rate(1 day)");
  t.truthy(schedules[0].handlers.length > 0);
  t.is(schedules[0].handlers[0].name, "cleanupHandler");
});

// ---------------------------------------------------------------------------
// 2. Consumer handler receives messages from Valkey
// ---------------------------------------------------------------------------

test("consumer handler receives messages from Valkey", async (t) => {
  const serverPort = PORT + 10;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  // Register a minimal HTTP handler so the server can start.
  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: "ok",
    }));
  }

  const received = deferred<JsConsumerEventInput>();

  // Register consumer handler.
  for (const consumer of config.consumers?.consumers ?? []) {
    for (const handler of consumer.handlers) {
      app.registerConsumerHandler(
        handler.name,
        handler.timeout,
        async (_err: Error | null, input: JsConsumerEventInput) => {
          received.resolve(input);
          return { success: true } as JsEventResult;
        },
      );
    }
  }

  await app.run(false);

  try {
    // Wait for the consumer to initialise its polling loop.
    await new Promise((r) => setTimeout(r, 1000));

    // Publish a message to the consumer stream.
    const { host, port } = parseRedisUrl(REDIS_URL);
    const redis = new Redis(port, host);
    try {
      const timestamp = Math.floor(Date.now() / 1000);
      await redis.xadd(
        "celerity:queue:test-order-queue",
        "*",
        "body",
        JSON.stringify({ orderId: "order-1", total: 42.5 }),
        "timestamp",
        String(timestamp),
        "message_type",
        "0",
      );

      // Wait for handler callback (timeout after 10s).
      const timer = setTimeout(() => received.reject(new Error("timeout waiting for consumer handler")), 10_000);
      const input = await received.promise;
      clearTimeout(timer);

      t.is(input.handlerTag, "source::test-order-queue::orderHandler");
      t.truthy(input.messages.length > 0);
      const msg = input.messages[0];
      t.truthy(msg.messageId);
      const body = JSON.parse(msg.body);
      t.is(body.orderId, "order-1");
      t.is(body.total, 42.5);
    } finally {
      await redis.quit();
    }
  } finally {
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 3. Consumer handler batch processing
// ---------------------------------------------------------------------------

test("consumer handler receives batch of messages", async (t) => {
  const serverPort = PORT + 11;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: "ok",
    }));
  }

  const received = deferred<JsConsumerEventInput>();

  for (const consumer of config.consumers?.consumers ?? []) {
    for (const handler of consumer.handlers) {
      app.registerConsumerHandler(
        handler.name,
        handler.timeout,
        async (_err: Error | null, input: JsConsumerEventInput) => {
          // Only resolve when we get a batch with multiple messages.
          if (input.messages.length >= 3) {
            received.resolve(input);
          }
          return { success: true } as JsEventResult;
        },
      );
    }
  }

  await app.run(false);

  try {
    await new Promise((r) => setTimeout(r, 1000));

    const { host, port } = parseRedisUrl(REDIS_URL);
    const redis = new Redis(port, host);
    try {
      const timestamp = String(Math.floor(Date.now() / 1000));

      // Publish 5 messages rapidly.
      for (let i = 0; i < 5; i++) {
        await redis.xadd(
          "celerity:queue:test-order-queue",
          "*",
          "body",
          JSON.stringify({ orderId: `batch-${i}` }),
          "timestamp",
          timestamp,
          "message_type",
          "0",
        );
      }

      const timer = setTimeout(() => received.reject(new Error("timeout waiting for batch")), 10_000);
      const input = await received.promise;
      clearTimeout(timer);

      t.true(input.messages.length >= 3);
    } finally {
      await redis.quit();
    }
  } finally {
    app.shutdown();
  }
});

// ---------------------------------------------------------------------------
// 4. Schedule handler receives trigger from Valkey
// ---------------------------------------------------------------------------

test("schedule handler receives trigger from Valkey", async (t) => {
  const serverPort = PORT + 12;
  const app = new CoreRuntimeApplication(testConfig({ serverPort }));
  const config = app.setup();

  for (const handler of config.api?.http?.handlers ?? []) {
    app.registerHttpHandler(handler.path, handler.method, null, async () => ({
      status: 200,
      body: "ok",
    }));
  }

  for (const consumer of config.consumers?.consumers ?? []) {
    for (const handler of consumer.handlers) {
      app.registerConsumerHandler(
        handler.name,
        handler.timeout,
        async () => ({ success: true }) as JsEventResult,
      );
    }
  }

  const received = deferred<JsScheduleEventInput>();

  for (const schedule of config.schedules?.schedules ?? []) {
    for (const handler of schedule.handlers) {
      app.registerScheduleHandler(
        handler.name,
        handler.timeout,
        async (_err: Error | null, input: JsScheduleEventInput) => {
          received.resolve(input);
          return { success: true } as JsEventResult;
        },
      );
    }
  }

  await app.run(false);

  try {
    await new Promise((r) => setTimeout(r, 1000));

    const { host, port } = parseRedisUrl(REDIS_URL);
    const redis = new Redis(port, host);
    try {
      const scheduleId = config.schedules!.schedules[0].scheduleId;
      const timestamp = String(Math.floor(Date.now() / 1000));

      await redis.xadd(
        `celerity:schedules:${scheduleId}`,
        "*",
        "body",
        JSON.stringify({ triggered: true }),
        "timestamp",
        timestamp,
        "message_type",
        "0",
      );

      const timer = setTimeout(() => received.reject(new Error("timeout waiting for schedule handler")), 10_000);
      const event = await received.promise;
      clearTimeout(timer);

      t.is(event.handlerTag, "source::dailyCleanup::cleanupHandler");
      t.truthy(event.messageId);
      t.truthy(event.scheduleId);
      // input comes from the blueprint schedule spec, not the message body.
      t.deepEqual(event.input, { task: "cleanup", enabled: true });
    } finally {
      await redis.quit();
    }
  } finally {
    app.shutdown();
  }
});
