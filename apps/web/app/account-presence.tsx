"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { accountRequest, type Profile } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";
import { ChatPanel } from "./chat-panel";
import { chatStore } from "./chat-store";
import { type ChatMessage, type ChatTarget } from "./chat-state";

export function AccountPresence({ compact = false }: { compact?: boolean }) {
  const router = useRouter();
  const [privateProfile, setPrivateProfile] = useState<Profile | null>(null);
  const [socialNotice, setSocialNotice] = useState<{
    name: string;
    sender: Profile;
    inviteCode?: string;
  } | null>(null);
  const [error, setError] = useState(false);
  const [notice, setNotice] = useState<{
    message: ChatMessage;
    target: ChatTarget;
  } | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    let socket: WebSocket | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let rotationTimer: ReturnType<typeof setTimeout> | undefined;
    let token = "";
    async function connect() {
      if (controller.signal.aborted || compact) return;
      try {
        const base =
          process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
        const response = await fetch(`${base}/v1/realtime/tickets`, {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          signal: controller.signal,
        });
        if (!response.ok) throw new Error("ticket failed");
        const ticket = (await response.json()) as { ticket: string };
        if (controller.signal.aborted) return;
        const url = new URL(`${base}/v1/ws`);
        url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
        url.searchParams.set("ticket", ticket.ticket);
        const next = new WebSocket(url);
        socket = next;
        next.onopen = () => {
          chatStore.attach(next);
          setError(false);
          window.dispatchEvent(new Event("chat-connected"));
          rotationTimer = setTimeout(
            () => next.close(4000, "rotation"),
            285000,
          );
        };
        next.onmessage = (event) => {
          try {
            chatStore.receive(
              JSON.parse(String(event.data)) as Record<string, unknown>,
            );
          } catch {}
        };
        next.onclose = () => {
          chatStore.detach(next);
          clearTimeout(rotationTimer);
          if (!controller.signal.aborted) {
            setError(true);
            retryTimer = setTimeout(
              () => void connect(),
              2000 + Math.random() * 1000,
            );
          }
        };
      } catch {
        if (!controller.signal.aborted) {
          setError(true);
          retryTimer = setTimeout(() => void connect(), 4000);
        }
      }
    }
    async function update() {
      if (running) return;
      running = true;
      try {
        await accountRequest("/heartbeat", {
          method: "POST",
          signal: controller.signal,
        });
      } catch {
        if (!controller.signal.aborted) setError(true);
      } finally {
        running = false;
      }
    }
    void accountRequest<{
      profile: Profile;
      credentials: { accessToken: string };
    }>("", { signal: controller.signal })
      .then((account) => {
        if (controller.signal.aborted) return;
        chatStore.identify(account.profile.id);
        token = account.credentials.accessToken;
        void connect();
      })
      .catch(() => {
        if (!controller.signal.aborted) setError(true);
      });
    void update();
    const interval = setInterval(() => void update(), 15000);
    window.addEventListener("online", update);
    return () => {
      controller.abort();
      clearInterval(interval);
      clearTimeout(retryTimer);
      clearTimeout(rotationTimer);
      if (socket) {
        chatStore.detach(socket);
        socket.close(1000, "page left");
      }
      window.removeEventListener("online", update);
    };
  }, [compact]);
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    function incoming(event: Event) {
      setSocialNotice(null);
      setNotice(
        (event as CustomEvent<{ message: ChatMessage; target: ChatTarget }>)
          .detail,
      );
      clearTimeout(timer);
      timer = setTimeout(() => setNotice(null), 8000);
    }
    function social(event: Event) {
      setNotice(null);
      setSocialNotice(
        (
          event as CustomEvent<{
            name: string;
            sender: Profile;
            inviteCode?: string;
          }>
        ).detail,
      );
      clearTimeout(timer);
      timer = setTimeout(() => setSocialNotice(null), 8000);
    }
    window.addEventListener("social-notice", social);
    window.addEventListener("chat-notice", incoming);
    return () => {
      window.removeEventListener("social-notice", social);
      window.removeEventListener("chat-notice", incoming);
      clearTimeout(timer);
    };
  }, []);
  return (
    <div className={compact ? "account-presence-compact" : "account-presence"}>
      {error ? (
        <p role="status" className="form-error">
          Koneksi akun terputus. Menghubungkan kembali…
        </p>
      ) : null}
      {notice ? (
        <div role="status" className="chat-toast">
          {notice.message.sender ? (
            <>
              <ProfileAvatar avatar={notice.message.sender.avatar} compact />
              <strong>{notice.message.sender.displayName}</strong>
            </>
          ) : null}
          <span>{notice.message.content}</span>
          <button
            type="button"
            onClick={() => {
              if (notice.target.scope === "private" && notice.message.sender)
                setPrivateProfile(notice.message.sender);
              else if (
                window.location.pathname === `/table/${notice.target.id}`
              )
                window.dispatchEvent(
                  new CustomEvent("open-table-chat", {
                    detail: notice.target.id,
                  }),
                );
              else router.push(`/table/${notice.target.id}?chat=open`);
              setNotice(null);
            }}
          >
            Open Chat
          </button>
          <button
            type="button"
            onClick={() => setNotice(null)}
            aria-label="Dismiss"
          >
            ×
          </button>
        </div>
      ) : null}
      {socialNotice ? (
        <div role="status" className="chat-toast">
          <ProfileAvatar avatar={socialNotice.sender.avatar} compact />
          <span>
            {socialNotice.name === "social.friend"
              ? "Kalian sekarang berteman"
              : `${socialNotice.sender.displayName} ${socialNotice.name === "social.follow" ? "mulai mengikuti Anda" : "mengundang Anda bermain"}`}
          </span>
          {socialNotice.name === "social.invite" ? (
            <Link
              href={`/lobby?invite=${encodeURIComponent(socialNotice.inviteCode ?? "")}`}
              onClick={() => setSocialNotice(null)}
            >
              Join
            </Link>
          ) : socialNotice.name === "social.friend" ? (
            <button
              type="button"
              onClick={() => {
                setPrivateProfile(socialNotice.sender);
                setSocialNotice(null);
              }}
            >
              Chat
            </button>
          ) : (
            <Link
              href={`/friends?user=${encodeURIComponent(socialNotice.sender.username)}`}
              onClick={() => setSocialNotice(null)}
            >
              View User
            </Link>
          )}
          <button type="button" onClick={() => setSocialNotice(null)}>
            Dismiss
          </button>
        </div>
      ) : null}
      {privateProfile ? (
        <div className="table-chat-container">
          <ChatPanel
            key={privateProfile.id}
            target={{ scope: "private", id: privateProfile.id }}
            profile={privateProfile}
            onClose={() => setPrivateProfile(null)}
          />
        </div>
      ) : null}
    </div>
  );
}
