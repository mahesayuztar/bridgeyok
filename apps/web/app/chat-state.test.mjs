import test from "node:test";
import assert from "node:assert/strict";
import { reconcileMessages, validChatContent } from "./chat-state.ts";

const pending = {
  messageId: "temp",
  clientRequestId: "request-123",
  senderUserId: "alice",
  content: "👩🏽‍💻",
  scope: "private",
  conversationId: "bob",
  createdAt: "2026-09-10T01:00:00Z",
  target: { scope: "private", id: "bob" },
  status: "sending",
};
test("ACK and reconnect reconcile request identity without duplicate or stale rollback", () => {
  const accepted = {
    ...pending,
    messageId: "server-id",
    status: "sent",
    conversationId: "alice:bob",
  };
  let messages = reconcileMessages([], pending);
  messages = reconcileMessages(messages, accepted);
  messages = reconcileMessages(messages, accepted);
  messages = reconcileMessages(messages, { ...pending, status: "failed" });
  assert.equal(messages.length, 1);
  assert.equal(messages[0].messageId, "server-id");
  assert.equal(messages[0].status, "sent");
  messages = reconcileMessages(messages, {
    ...accepted,
    messageId: "bob-message",
    senderUserId: "bob",
  });
  assert.equal(messages.length, 2);
});
test("Unicode graphemes preserve emoji, skin tones, ZWJ and suits", () => {
  for (const content of [
    "👩🏽‍💻".repeat(1000),
    "e\u0301".repeat(1000),
    "♠️ ♥️ ♦️ ♣️",
    "😀👍🏽",
  ])
    assert.equal(validChatContent(content), true);
  for (const content of [
    "😀".repeat(1001),
    " ",
    "\0",
    "a" + "\u0301".repeat(9000),
  ])
    assert.equal(validChatContent(content), false);
});
