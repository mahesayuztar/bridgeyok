"use client";

import { accountRequest } from "./account-types";
import {
  reconcileMessages,
  validChatContent,
  type ChatMessage,
  type ChatTarget,
  type LocalMessage,
  type ChatPage,
} from "./chat-state";

const listeners = new Set<() => void>();
const emptyMessages: LocalMessage[] = [];
let messages: LocalMessage[] = emptyMessages;
let socket: WebSocket | null = null;
let viewerId = "";
let openTarget: ChatTarget | null = null;
const timers = new Map<string, ReturnType<typeof setTimeout>>();
const seen = new Set<string>();

export const chatStore = {
  subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
    };
  },
  snapshot() {
    return messages;
  },
  serverSnapshot() {
    return emptyMessages;
  },
  identify(id: string) {
    if (viewerId !== id) {
      messages = [];
      seen.clear();
      for (const timer of timers.values()) clearTimeout(timer);
      timers.clear();
    }
    viewerId = id;
    listeners.forEach((listener) => listener());
  },
  open(target: ChatTarget | null) {
    openTarget = target;
  },
  attach(next: WebSocket) {
    socket = next;
  },
  detach(previous: WebSocket) {
    if (socket === previous) socket = null;
  },
  receive(envelope: Record<string, unknown>) {
    if (typeof envelope.name !== "string") return false;
    const pending =
      typeof envelope.request_id === "string" &&
      messages.find(
        (message) =>
          message.clientRequestId === envelope.request_id &&
          message.status !== "sent",
      );
    if (envelope.kind === "error" && pending) {
      messages = reconcileMessages(messages, { ...pending, status: "failed" });
      listeners.forEach((listener) => listener());
      return true;
    }
    if (envelope.name.startsWith("social.")) {
      const payload = envelope.payload as
        { eventId?: string; sender?: { id: string } } | undefined;
      if (
        payload?.eventId &&
        !seen.has(payload.eventId) &&
        payload.sender?.id !== viewerId
      ) {
        seen.add(payload.eventId);
        if (seen.size > 2000) seen.delete(seen.values().next().value!);
        window.dispatchEvent(
          new CustomEvent("social-notice", {
            detail: { name: envelope.name, ...payload },
          }),
        );
      }
      return true;
    }
    if (!envelope.name.startsWith("chat.")) return false;
    const payload = envelope.payload as { message?: ChatMessage } | undefined;
    const message = payload?.message;
    if (
      !message ||
      typeof message.messageId !== "string" ||
      typeof message.content !== "string" ||
      typeof message.senderUserId !== "string" ||
      typeof message.conversationId !== "string" ||
      (message.scope !== "private" && message.scope !== "table")
    )
      return true;
    const target: ChatTarget = {
      scope: message.scope,
      id:
        message.scope === "table"
          ? message.conversationId
          : (message.conversationId.split(":").find((id) => id !== viewerId) ??
            ""),
    };
    clearTimeout(timers.get(message.clientRequestId));
    timers.delete(message.clientRequestId);
    messages = reconcileMessages(messages, {
      ...message,
      target,
      status: "sent",
    });
    listeners.forEach((listener) => listener());
    if (
      !seen.has(message.messageId) &&
      message.senderUserId !== viewerId &&
      !(openTarget?.scope === target.scope && openTarget.id === target.id)
    ) {
      window.dispatchEvent(
        new CustomEvent("chat-notice", { detail: { message, target } }),
      );
    }
    seen.add(message.messageId);
    if (seen.size > 2000) seen.delete(seen.values().next().value!);
    return true;
  },
  send(target: ChatTarget, content: string, requestId = crypto.randomUUID()) {
    if (!validChatContent(content) || !viewerId) return;
    const previous = messages.find(
      (message) =>
        message.clientRequestId === requestId &&
        message.senderUserId === viewerId,
    );
    if (previous?.status === "sent") return;
    const message: LocalMessage = previous
      ? { ...previous, status: "retrying" }
      : {
          messageId: requestId,
          clientRequestId: requestId,
          senderUserId: viewerId,
          scope: target.scope,
          conversationId: target.id,
          target,
          content,
          createdAt: new Date().toISOString(),
          status: "sending",
        };
    messages = reconcileMessages(messages, message);
    listeners.forEach((listener) => listener());
    clearTimeout(timers.get(requestId));
    const fail = () => {
      timers.delete(requestId);
      messages = reconcileMessages(messages, { ...message, status: "failed" });
      listeners.forEach((listener) => listener());
    };
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      fail();
      return;
    }
    try {
      socket.send(
        JSON.stringify({
          v: 1,
          kind: "command",
          name: `chat.${target.scope}.send`,
          request_id: requestId,
          payload: { target, content },
        }),
      );
      timers.set(requestId, setTimeout(fail, 10000));
    } catch {
      fail();
    }
  },
  async history(target: ChatTarget, cursor = "", signal?: AbortSignal) {
    const page = await accountRequest<ChatPage>(
      `/chat?scope=${target.scope}&id=${encodeURIComponent(target.id)}&cursor=${encodeURIComponent(cursor)}`,
      { signal: signal ?? null },
    );
    if (signal?.aborted) return page;
    for (const message of page.messages) {
      seen.add(message.messageId);
      if (seen.size > 2000) seen.delete(seen.values().next().value!);
      messages = reconcileMessages(messages, {
        ...message,
        target,
        status: "sent",
      });
    }
    listeners.forEach((listener) => listener());
    return page;
  },
};
