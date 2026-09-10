"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { ChatPanel } from "../chat-panel";

export function TableChat({ tableId }: { tableId: string }) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const search = useSearchParams();
  const [selectedOpen, setOpen] = useState<boolean | null>(null);
  const open = selectedOpen ?? search.get("chat") === "open";
  useEffect(() => {
    function show(event: Event) {
      if ((event as CustomEvent<string>).detail === tableId) setOpen(true);
    }
    window.addEventListener("open-table-chat", show);
    return () => window.removeEventListener("open-table-chat", show);
  }, [tableId]);
  useEffect(() => {
    document.body.classList.toggle("table-chat-open", open);
    return () => document.body.classList.remove("table-chat-open");
  }, [open]);
  return (
    <>
      <button type="button" onClick={() => setOpen(!open)} aria-expanded={open}>
        Chat
      </button>
      {open ? (
        <div className="table-chat-container">
          <ChatPanel
            target={{ scope: "table", id: tableId }}
            onClose={() => {
              setOpen(false);
              triggerRef.current?.focus();
            }}
          />
        </div>
      ) : null}
    </>
  );
}
