import type { Avatar } from "./account-types";

const symbols: Record<Avatar, string> = { spade: "♠", heart: "♥", diamond: "♦", club: "♣", owl: "🦉", fox: "🦊" };

export function ProfileAvatar({ avatar, online, compact = false }: { avatar: Avatar; online?: boolean; compact?: boolean }) {
  return <span className={`profile-avatar${compact ? " profile-avatar-compact" : ""}`} data-avatar={avatar}>
    <span aria-hidden="true">{symbols[avatar]}</span>
    {online === undefined ? null : <span className="presence-dot" data-online={online} role="img" aria-label={online ? "Online" : "Offline"} />}
  </span>;
}
