import assert from "node:assert/strict";
import test from "node:test";
import { createRequestId } from "./request-id.ts";

test("creates unique contract-safe request ids without randomUUID", () => {
  const requestIds = Array.from({ length: 64 }, () => createRequestId());

  assert.equal(new Set(requestIds).size, requestIds.length);
  for (const requestId of requestIds) {
    assert.match(requestId, /^req_[A-Za-z0-9_-]+$/);
  }
});
