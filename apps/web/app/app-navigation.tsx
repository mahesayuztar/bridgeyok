"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type KeyboardEvent } from "react";
import { ProfileAvatar } from "./profile-avatar";
import type { Profile } from "./account-types";
import { useTableSession } from "./use-table-session";
import { useTurnSoundPreference } from "./use-turn-audio";
import { turnCueState } from "./turn-cue";

function NavigationIcon({ name }: { name: "table" | "friends" | "history" | "more" | "settings" }) {
  if (name === "table") return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6.5 3.5h8l3 3v14h-11z" /><path d="M14.5 3.5v3h3M9.5 11.5h5M9.5 15.5h5" /></svg>;
  if (name === "friends") return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="9" cy="8" r="3" /><path d="M3.5 19c.5-4 2.3-6 5.5-6s5 2 5.5 6M15 6.5a3 3 0 0 1 0 5.8M16 14c2.7.3 4.1 2 4.5 5" /></svg>;
  if (name === "history") return <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 8V4m0 0h4M4 4l3 3a8 8 0 1 1-2 8" /><path d="M12 8v4l3 2" /></svg>;
  if (name === "settings") return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3" /><path d="M19 13.5v-3l-2-.5a7 7 0 0 0-.7-1.6l1.1-1.8-2.1-2.1-1.8 1.1a7 7 0 0 0-1.6-.7l-.5-2h-3l-.5 2a7 7 0 0 0-1.6.7L4.5 4.5 2.4 6.6l1.1 1.8A7 7 0 0 0 2.8 10l-2 .5v3l2 .5a7 7 0 0 0 .7 1.6l-1.1 1.8 2.1 2.1 1.8-1.1a7 7 0 0 0 1.6.7l.5 2h3l.5-2a7 7 0 0 0 1.6-.7l1.8 1.1 2.1-2.1-1.1-1.8A7 7 0 0 0 19 14z" /></svg>;
  return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5" cy="12" r="1.5" /><circle cx="12" cy="12" r="1.5" /><circle cx="19" cy="12" r="1.5" /></svg>;
}

export function AppNavigation({ profile }: { profile: Profile }) {
  const pathname = usePathname();
  const session = useTableSession();
  const sound = useTurnSoundPreference();
  const [logoutError, setLogoutError] = useState(false);
  const [dismissedTooltip, setDismissedTooltip] = useState<string | null>(null);
  const table = session.projectedTable;
  const activeTableId = session.tableState.activeTableId;
  const tableHref = activeTableId === null ? "/play" : `/table/${activeTableId}`;
  const tableLabel = activeTableId === null ? "Play" : "Table";
  const tableActive = pathname === tableHref || (activeTableId === null && (pathname === "/lobby" || pathname.startsWith("/match")));
  const viewerTurn = table === null ? false : (turnCueState(table)?.viewerTurn ?? false);
  const boardNumber = table?.game?.board.number ?? (table?.boardNumber && table.boardNumber > 0 ? table.boardNumber : undefined);

  function dismissTooltip(event: KeyboardEvent<HTMLElement>, name: string) {
    if (event.key === "Escape") setDismissedTooltip(name);
  }

  async function logout() {
    setLogoutError(false);
    if (await session.logout()) window.location.replace("/login");
    else setLogoutError(true);
  }

  return (
    <aside className="account-navigation">
      <Link href="/play" className="wordmark" aria-label="BridgeYok">BY</Link>
      <nav aria-label="Navigasi utama">
        <Link href={tableHref} className="app-nav-item nav-table" data-tooltip={tableLabel} data-tooltip-dismissed={dismissedTooltip === "table"} aria-label={viewerTurn ? `${tableLabel}, giliran Anda${boardNumber === undefined ? "" : `, board ${boardNumber}`}` : tableLabel} aria-current={tableActive ? "page" : undefined} onKeyDown={(event) => dismissTooltip(event, "table")} onPointerLeave={() => setDismissedTooltip(null)} onBlur={() => setDismissedTooltip(null)}>
          <NavigationIcon name="table" />
          <span className="nav-label">{tableLabel}</span>
          {activeTableId === null ? null : <span className="nav-table-status" data-turn={viewerTurn} aria-hidden="true">{boardNumber === undefined ? "Live" : `B${boardNumber}`}</span>}
        </Link>
        <Link href="/friends" className="app-nav-item" data-tooltip="Friends" data-tooltip-dismissed={dismissedTooltip === "friends"} aria-label="Friends" aria-current={pathname === "/friends" ? "page" : undefined} onKeyDown={(event) => dismissTooltip(event, "friends")} onPointerLeave={() => setDismissedTooltip(null)} onBlur={() => setDismissedTooltip(null)}>
          <NavigationIcon name="friends" /><span className="nav-label">Friends</span>
        </Link>
        <Link href="/history" className="app-nav-item" data-tooltip="History" data-tooltip-dismissed={dismissedTooltip === "history"} aria-label="History" aria-current={pathname === "/history" ? "page" : undefined} onKeyDown={(event) => dismissTooltip(event, "history")} onPointerLeave={() => setDismissedTooltip(null)} onBlur={() => setDismissedTooltip(null)}>
          <NavigationIcon name="history" /><span className="nav-label">History</span>
        </Link>
        <button type="button" className="app-nav-item" data-tooltip="More" data-tooltip-dismissed={dismissedTooltip === "more"} aria-label="More" aria-current={pathname === "/settings" ? "page" : undefined} popoverTarget="app-more-menu" onKeyDown={(event) => dismissTooltip(event, "more")} onPointerLeave={() => setDismissedTooltip(null)} onBlur={() => setDismissedTooltip(null)}>
          <NavigationIcon name="more" /><span className="nav-label">More</span>
        </button>
      </nav>
      <div id="app-more-menu" className="app-more-menu" popover="auto">
        <div className="app-more-profile"><ProfileAvatar avatar={profile.avatar} compact /><div><strong>{profile.displayName}</strong><span>@{profile.username}</span></div></div>
        <Link href="/settings" className="app-more-link" onClick={() => document.getElementById("app-more-menu")?.hidePopover()}><NavigationIcon name="settings" /><span>Settings dan profile</span></Link>
        <label className="app-more-toggle"><input type="checkbox" checked={!sound.muted} onChange={(event) => sound.setMuted(!event.target.checked)} />Suara giliran</label>
        <details className="app-help">
          <summary>Bantuan bermain</summary>
          <p>Pilih kartu atau bid hanya saat giliranmu. Di auction, gunakan P untuk Pass, X untuk Double, dan R untuk Redouble saat meja sedang aktif.</p>
        </details>
        <button type="button" className="app-more-logout" disabled={session.busy} onClick={() => void logout()}>{session.busy ? "Menutup sesi…" : "Logout"}</button>
        {logoutError ? <p className="form-error" role="alert">Logout gagal. Sesi dan meja tetap aktif; coba lagi.</p> : null}
      </div>
    </aside>
  );
}
