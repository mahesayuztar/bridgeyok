import type { Profile } from "./account-types";
import type { components } from "@bridgeyok/contracts/openapi";
export type ChatTarget = { scope: "private" | "table"; id: string };
export type ChatMessage = components["schemas"]["ChatMessage"];
export type LocalMessage = Omit<ChatMessage, "sender"> & {
  sender?: Profile;
  status: "sending" | "sent" | "failed" | "retrying";
  target: ChatTarget;
};
export type ChatPage = components["schemas"]["ChatPage"];

export function reconcileMessages(
  messages: LocalMessage[],
  incoming: LocalMessage,
): LocalMessage[] {
  const previous = messages.find(
    (message) =>
      message.messageId === incoming.messageId ||
      (message.senderUserId === incoming.senderUserId &&
        message.clientRequestId === incoming.clientRequestId),
  );
  const next =
    previous?.status === "sent" && incoming.status !== "sent"
      ? previous
      : incoming;
  return [...messages.filter((message) => message !== previous), next].sort(
    (first, second) =>
      first.createdAt.localeCompare(second.createdAt) ||
      first.messageId.localeCompare(second.messageId),
  );
}

export function validChatContent(content: string) {
  return (
    content.trim().length > 0 &&
    !content.includes("\0") &&
    new TextEncoder().encode(content).length <= 16000 &&
    Array.from(
      new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(
        content,
      ),
    ).length <= 1000
  );
}
