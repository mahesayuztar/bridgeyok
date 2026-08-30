import assert from "node:assert/strict";
import test from "node:test";
import { issueFromServer } from "./client-issue.ts";

test("server issue classifier keeps table failures distinct", () => {
  assert.equal(issueFromServer({ code: "TABLE_NOT_FOUND", source: "rest" }).kind, "notFound");
  assert.equal(issueFromServer({ code: "TABLE_UNAVAILABLE", source: "rest" }).kind, "unavailable");
  assert.equal(issueFromServer({ code: "TABLE_FULL", source: "rest" }).kind, "full");
  assert.equal(issueFromServer({ code: "TABLE_LOCKED", source: "rest" }).kind, "locked");
  assert.equal(issueFromServer({ status: 503, source: "rest" }).kind, "server");
});

test("controller errors always request an explicit resync", () => {
  const stale = issueFromServer({ code: "STALE_CONTROLLER", source: "websocket" });
  const changed = issueFromServer({ code: "STATE_CHANGED", source: "websocket" });
  const malformed = issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "websocket" });
  const malformedResponse = issueFromServer({ code: "INVALID_TABLE_PROJECTION", source: "rest" });
  assert.equal(stale.action, "resync");
  assert.equal(changed.action, "resync");
  assert.equal(malformed.action, "resync");
  assert.equal(malformedResponse.action, "retry");
});
