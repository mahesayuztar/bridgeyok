import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const packageDirectory = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const schema = JSON.parse(await readFile(path.join(packageDirectory, "websocket/envelope.schema.json"), "utf8"));
const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);
const validateEnvelope = ajv.compile(schema);

test("WebSocket examples satisfy the envelope contract", async () => {
  const examplesDirectory = path.join(packageDirectory, "websocket/examples");
  const exampleFiles = (await readdir(examplesDirectory)).filter((fileName) => fileName.endsWith(".json"));

  assert.ok(exampleFiles.length > 0);
  for (const fileName of exampleFiles) {
    const example = JSON.parse(await readFile(path.join(examplesDirectory, fileName), "utf8"));
    assert.equal(validateEnvelope(example), true, `${fileName}: ${ajv.errorsText(validateEnvelope.errors)}`);
  }
});

test("WebSocket contract rejects unsafe envelopes", () => {
  const invalidEnvelopes = [
    { v: 2, kind: "control", name: "heartbeat", payload: {} },
    { v: 1, kind: "command", name: "table.join", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", payload: {} },
    { v: 1, kind: "event", name: "table.updated", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", revision: -1, seq: 1, payload: {} },
    { v: 1, kind: "control", name: "heartbeat", payload: {}, unexpected: true }
  ];

  for (const envelope of invalidEnvelopes) {
    assert.equal(validateEnvelope(envelope), false);
  }
});
