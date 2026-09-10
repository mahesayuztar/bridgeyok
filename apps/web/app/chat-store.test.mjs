import test from "node:test";
import assert from "node:assert/strict";
import { chatStore } from "./chat-store.ts";

const target = { scope: "private", id: "bob" };
const accepted = {
  messageId: "server-id",
  scope: "private",
  conversationId: "alice:bob",
  senderUserId: "alice",
  clientRequestId: "request-123",
  content: "👩🏽‍💻",
  createdAt: "2026-09-11T00:00:00Z",
};

test("retry keeps request identity; late ACK and replay reconcile once", (t) => {
  globalThis.window = new EventTarget();
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const sent = [];
  chatStore.identify("alice");
  chatStore.attach({
    readyState: 1,
    send: (frame) => sent.push(JSON.parse(frame)),
  });
  chatStore.send(target, "👩🏽‍💻", "request-123");
  assert.equal(chatStore.snapshot()[0].status, "sending");
  t.mock.timers.tick(10000);
  assert.equal(chatStore.snapshot()[0].status, "failed");
  chatStore.send(target, "👩🏽‍💻", "request-123");
  assert.equal(chatStore.snapshot()[0].status, "retrying");
  assert.equal(sent[0].request_id, sent[1].request_id);
  chatStore.receive({
    name: "chat.accepted",
    kind: "control",
    payload: { message: accepted },
  });
  chatStore.receive({
    name: "chat.private.received",
    kind: "control",
    payload: { message: accepted },
  });
  t.mock.timers.tick(10000);
  assert.equal(chatStore.snapshot().length, 1);
  assert.equal(chatStore.snapshot()[0].status, "sent");
  chatStore.identify("");
  assert.equal(chatStore.snapshot().length, 0);
});

test("notifications suppress self, open conversations and duplicate chat/social events", () => {
  globalThis.window = new EventTarget();
  chatStore.identify("alice");
  let chatNotices = 0;
  let socialNotices = 0;
  window.addEventListener("chat-notice", () => chatNotices++);
  window.addEventListener("social-notice", () => socialNotices++);
  chatStore.receive({
    name: "chat.private.received",
    kind: "control",
    payload: { message: accepted },
  });
  assert.equal(chatNotices, 0);
  const incoming = {
    ...accepted,
    messageId: "bob-message",
    senderUserId: "bob",
  };
  chatStore.open(target);
  chatStore.receive({
    name: "chat.private.received",
    kind: "control",
    payload: { message: incoming },
  });
  assert.equal(chatNotices, 0);
  chatStore.open(null);
  chatStore.receive({
    name: "chat.private.received",
    kind: "control",
    payload: { message: incoming },
  });
  assert.equal(chatNotices, 0);
  const next = {
    ...incoming,
    messageId: "bob-next",
    clientRequestId: "request-next",
  };
  for (let _index = 0; _index < 3; _index++)
    chatStore.receive({
      name: "chat.private.received",
      kind: "control",
      payload: { message: next },
    });
  assert.equal(chatNotices, 1);
  for (const name of ["social.follow", "social.friend", "social.invite"]) {
    for (let _index = 0; _index < 3; _index++)
      chatStore.receive({
        name,
        payload: { eventId: name, sender: { id: "bob" } },
      });
  }
  assert.equal(socialNotices, 3);
  chatStore.receive({
    name: "social.follow",
    payload: { eventId: "self", sender: { id: "alice" } },
  });
  assert.equal(socialNotices, 3);
  chatStore.identify("");
});

test("late history and replaced sockets cannot reintroduce another account's messages", async (t) => {
  globalThis.window = new EventTarget();
  chatStore.identify("alice");
  const previous = { readyState: 1, send() {} };
  const current = { readyState: 1, send() {} };
  chatStore.attach(previous);
  chatStore.attach(current);
  chatStore.receive(
    { name: "chat.accepted", payload: { message: accepted } },
    previous,
  );
  assert.equal(chatStore.snapshot().length, 0);
  let finish;
  t.mock.method(
    globalThis,
    "fetch",
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const pending = chatStore.history(target);
  chatStore.identify("eve");
  finish(Response.json({ messages: [accepted] }));
  await pending;
  assert.equal(chatStore.snapshot().length, 0);
  chatStore.identify("");
  chatStore.receive({ name: "chat.accepted", payload: { message: accepted } });
  assert.equal(chatStore.snapshot().length, 0);
});
