"use client";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { chatStore } from "./chat-store";
import { validChatContent, type ChatTarget } from "./chat-state";
import { ProfileAvatar } from "./profile-avatar";
import { accountRequest, type Profile } from "./account-types";

export function ChatPanel({
  target,
  profile,
  onClose,
}: {
  target: ChatTarget;
  profile?: Profile;
  onClose: () => void;
}) {
  const messages = useSyncExternalStore(
    chatStore.subscribe,
    chatStore.snapshot,
    chatStore.serverSnapshot,
  ).filter(
    (message) =>
      message.target.scope === target.scope && message.target.id === target.id,
  );
  const [liveProfile, setLiveProfile] = useState<Profile | null>(null);
  const currentProfile =
    liveProfile?.id === profile?.id ? liveProfile : profile;
  const canSend =
    target.scope !== "private" || liveProfile === null || liveProfile.friends;
  const username = profile?.username;
  const [content, setContent] = useState("");
  const [cursor, setCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [unread, setUnread] = useState(false);
  const panelRef = useRef<HTMLElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const nearBottom = useRef(true);
  const lastIdentity = messages.at(-1)?.messageId;
  const { scope, id } = target;
  useEffect(() => {
    if (scope !== "private" || !username) return;
    const controller = new AbortController();
    let running = false;
    async function updateProfile() {
      if (running) return;
      running = true;
      try {
        const profiles = await accountRequest<Profile[]>(
          `/users?q=${encodeURIComponent(username!)}&friends=false`,
          { signal: controller.signal },
        );
        const next = profiles.find((profile) => profile.id === id);
        if (next && !controller.signal.aborted) setLiveProfile(next);
      } catch {
      } finally {
        running = false;
      }
    }
    void updateProfile();
    const interval = setInterval(() => void updateProfile(), 15000);
    return () => {
      controller.abort();
      clearInterval(interval);
    };
  }, [scope, id, username]);
  useEffect(() => {
    const target = { scope, id };
    chatStore.open(target);
    const controller = new AbortController();
    async function load() {
      try {
        const page = await chatStore.history(target, "", controller.signal);
        if (!controller.signal.aborted) {
          setCursor(page.nextCursor ?? "");
          setError("");
        }
      } catch {
        if (!controller.signal.aborted)
          setError("Riwayat belum terhubung. Coba lagi.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    window.addEventListener("chat-connected", load);
    return () => {
      controller.abort();
      chatStore.open(null);
      window.removeEventListener("chat-connected", load);
    };
  }, [scope, id]);
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    if (nearBottom.current) list.scrollTop = list.scrollHeight;
    else setUnread(true);
  }, [lastIdentity]);
  useEffect(() => {
    const viewport = window.visualViewport;
    const container = panelRef.current?.closest<HTMLElement>(
      ".table-chat-container",
    );
    if (!viewport || !container) return;
    function updateViewport() {
      container!.style.setProperty(
        "--chat-keyboard-offset",
        `${Math.max(0, innerHeight - viewport!.height - viewport!.offsetTop)}px`,
      );
      container!.style.setProperty(
        "--chat-visible-height",
        `${viewport!.height}px`,
      );
    }
    updateViewport();
    viewport.addEventListener("resize", updateViewport);
    viewport.addEventListener("scroll", updateViewport);
    return () => {
      viewport.removeEventListener("resize", updateViewport);
      viewport.removeEventListener("scroll", updateViewport);
      container.style.removeProperty("--chat-keyboard-offset");
      container.style.removeProperty("--chat-visible-height");
    };
  }, []);
  async function older() {
    const list = listRef.current;
    const height = list?.scrollHeight ?? 0;
    setLoading(true);
    setError("");
    try {
      const page = await chatStore.history(target, cursor);
      setCursor(page.nextCursor ?? "");
      requestAnimationFrame(() => {
        if (list) list.scrollTop += list.scrollHeight - height;
      });
    } catch {
      setError("Riwayat belum terhubung. Coba lagi.");
    } finally {
      setLoading(false);
    }
  }
  return (
    <section
      ref={panelRef}
      className="chat-panel"
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          event.stopPropagation();
          onClose();
        }
      }}
      aria-label={
        currentProfile ? `Chat ${currentProfile.displayName}` : "Chat meja"
      }
    >
      <header>
        {currentProfile ? (
          <ProfileAvatar
            avatar={currentProfile.avatar}
            online={currentProfile.online}
            compact
          />
        ) : null}
        <div>
          <strong>{currentProfile?.displayName ?? "Chat meja"}</strong>
          {currentProfile ? (
            <small>
              {currentProfile.online
                ? "Online"
                : "Offline · pesan tetap terkirim"}
            </small>
          ) : (
            <small>Participant meja ini · 14 hari</small>
          )}
        </div>
        <button type="button" onClick={onClose} aria-label="Tutup chat">
          ×
        </button>
      </header>
      <div
        className="chat-messages"
        ref={listRef}
        onScroll={() => {
          const list = listRef.current;
          if (list) {
            nearBottom.current =
              list.scrollHeight - list.scrollTop - list.clientHeight < 64;
            if (nearBottom.current) setUnread(false);
          }
        }}
      >
        {cursor || error ? (
          <button disabled={loading} type="button" onClick={() => void older()}>
            Muat riwayat
          </button>
        ) : null}
        {loading ? <p role="status">Memuat pesan…</p> : null}
        {error ? <p role="alert">{error}</p> : null}
        {messages.map((message) => (
          <div
            className="chat-message"
            key={`${message.senderUserId}:${message.clientRequestId}`}
          >
            {message.sender ? (
              <strong>{message.sender.displayName}</strong>
            ) : (
              <strong>Kamu</strong>
            )}
            <p>{message.content}</p>
            <small>
              <time dateTime={message.createdAt}>
                {new Date(message.createdAt).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </time>{" "}
              ·{" "}
              {message.status === "sent"
                ? "Terkirim"
                : message.status === "failed"
                  ? "Gagal"
                  : message.status === "retrying"
                    ? "Mencoba lagi…"
                    : "Mengirim…"}
            </small>
            {message.status === "failed" ? (
              <button
                type="button"
                onClick={() =>
                  chatStore.send(
                    target,
                    message.content,
                    message.clientRequestId,
                  )
                }
              >
                Coba lagi
              </button>
            ) : null}
          </div>
        ))}
      </div>
      {unread ? (
        <button
          type="button"
          onClick={() => {
            if (listRef.current)
              listRef.current.scrollTop = listRef.current.scrollHeight;
            nearBottom.current = true;
            setUnread(false);
          }}
        >
          Pesan baru ↓
        </button>
      ) : null}
      {!canSend ? (
        <p role="status">Chat hanya tersedia untuk Friends.</p>
      ) : null}
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!canSend || !validChatContent(content)) return;
          chatStore.send(target, content);
          setContent("");
          nearBottom.current = true;
        }}
      >
        <textarea
          aria-label="Pesan"
          disabled={!canSend}
          placeholder="Tulis pesan… 😀"
          value={content}
          onChange={(event) => setContent(event.target.value)}
          rows={2}
        />
        <button type="submit" disabled={!canSend || !validChatContent(content)}>
          Kirim
        </button>
      </form>
    </section>
  );
}
