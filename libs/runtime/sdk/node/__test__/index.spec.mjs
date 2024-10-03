import test from "ava";
import { promisify } from "node:util";

import { CoreRuntimeApplication } from "../index.js";
import supertest from "supertest";

const request = supertest("http://localhost:22345");

/**
 *
 * @param {(t: import('ava').ExecutionContext, end: (err?: null | Error) => void)} fn
 * @returns {(t: import('ava').ExecutionContext) => Promise<void>}
 */
const withCallback = (fn) => async (t) => {
  await promisify(fn)(t);
  t.pass(); // There must be at least one passing assertion for the test to pass
};

test.afterEach(() => {
  if (app) {
    app.shutdown();
  }
});

/**
 * @type {CoreRuntimeApplication}
 */
let app;

test(
  "starts up core runtime http server",
  withCallback((t, end) => {
    app = new CoreRuntimeApplication({
      blueprintConfigPath: "__test__/http-api.blueprint.yaml",
      serverPort: 22345,
      serverLoopbackOnly: true,
    });
    const appConfig = app.setup();
    console.log({ appConfig });
    for (const handlerDef of appConfig.api?.http?.handlers ?? []) {
      console.log({ handlerDef });
      app.registerHttpHandler(
        handlerDef.path,
        handlerDef.method,
        async (_, request) => {
          console.log("Called JS HTTP handler!");
          console.log(await request.body());
          console.log(request.headers());
          console.log(request.httpVersion());
          return {
            status: 200,
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ message: "Hello, world!" }),
          };
        }
      );
    }
    app
      .run()
      .then(() => {
        request
          .post("/orders/1")
          .send({ order: { id: 1 } })
          .expect("Content-Type", /json/)
          .expect(200)
          .end((err, res) => {
            if (err) {
              return end(err);
            }

            t.deepEqual(JSON.parse(res.text), { message: "Hello, world!" });
            return end();
          });
      })
      .catch((err) => end(err));
  })
);
