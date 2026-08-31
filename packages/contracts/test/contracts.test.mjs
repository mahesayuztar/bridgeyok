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
    { v: 1, kind: "command", name: "table.unknown", request_id: "request_01", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", expected_revision: 0, payload: {} },
    { v: 1, kind: "command", name: "table.set_ready", request_id: "request_02", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", expected_revision: 0, controller_epoch: 0, payload: { ready: true } },
    { v: 1, kind: "command", name: "table.take_seat", request_id: "request_03", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", expected_revision: 0, payload: { seat: "N", hidden: true } },
    { v: 1, kind: "event", name: "table.updated", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", revision: -1, seq: 1, payload: {} },
    { v: 1, kind: "control", name: "presence.snapshot", table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31", payload: { participants: [{ participantId: realtimeParticipantId, online: false, expiresAt: "soon" }] } },
    { v: 1, kind: "control", name: "heartbeat", payload: {}, unexpected: true }
  ];

  for (const envelope of invalidEnvelopes) {
    assert.equal(validateEnvelope(envelope), false);
  }
});

test("WebSocket contract accepts owner bot mutations", () => {
  const commands = [
    { name: "table.add_bot", payload: { seat: "N" } },
    { name: "table.remove_bot", payload: { seat: "E" } },
    { name: "table.replace_with_bot", payload: { participant_id: realtimeParticipantId } }
  ];

  for (const [_commandIndex, command] of commands.entries()) {
    const envelope = {
      v: 1,
      kind: "command",
      name: command.name,
      request_id: `bot_command_${_commandIndex}`,
      table_id: "0f4b9a5b-0ea8-4ad6-a866-e576ccd8be31",
      expected_revision: _commandIndex,
      controller_epoch: 1,
      payload: command.payload
    };
    assert.equal(validateEnvelope(envelope), true, ajv.errorsText(validateEnvelope.errors));
  }
});

const realtimeParticipantId = "99ef3682-3ba8-42db-9c33-17238bfb2207";
