"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { type Profile } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";
import { ChatPanel } from "./chat-panel";
import { type ChatMessage, type ChatTarget } from "./chat-state";
import { useTableSession } from "./use-table-session";

export function AccountPresence({ compact = false }: { compact?: boolean }) {
  const router = useRouter();
  const pathname = usePathname();
  const session = useTableSession();
  const [privateProfile, setPrivateProfile] = useState<Profile | null>(null);
  const [socialNotice, setSocialNotice] = useState<{
    name: string;
    sender: Profile;
    inviteCode?: string;
  } | null>(null);
  const [notice, setNotice] = useState<{
    message: ChatMessage;
    target: ChatTarget;
  } | null>(null);
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
      {session.nickname !== null && (session.connectionState === "degraded" || session.connectionState === "offline") ? (
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
          {notice.target.scope === "table" && pathname === `/table/${notice.target.id}` ? (
            <span>Lihat di panel Chat</span>
          ) : (
            <button
              type="button"
              onClick={() => {
                if (notice.target.scope === "private" && notice.message.sender)
                  setPrivateProfile(notice.message.sender);
                else router.push(`/table/${notice.target.id}`);
                setNotice(null);
              }}
            >
              Open Chat
            </button>
          )}
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
        <div className="private-chat-container">
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
