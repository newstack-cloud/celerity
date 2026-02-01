import { CelerityRuntime } from "@celerity-sdk/runtime";

async function run() {
    const runtime = new CelerityRuntime();
    await runtime.start();
}

run();
