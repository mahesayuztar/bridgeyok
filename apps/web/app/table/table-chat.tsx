"use client";

import { ChatPanel } from "../chat-panel";

export function TableChat({ tableId }: { tableId: string }) {
  return (
    <aside className="play-board-chat" aria-label="Panel chat">
      <ChatPanel target={{ scope: "table", id: tableId }} />
    </aside>
  );
}
