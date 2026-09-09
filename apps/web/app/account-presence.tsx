"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { accountRequest, type Invitation } from "./account-types";
import { ProfileAvatar } from "./profile-avatar";

export function AccountPresence({ compact = false }: { compact?: boolean }) {
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [error, setError] = useState(false);
  const [hidden, setHidden] = useState<string[]>([]);
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    async function update() {
      if (running) return;
      running = true;
      try {
        await accountRequest("/heartbeat", { method: "POST", signal: controller.signal });
        const next = await accountRequest<Invitation[]>("/invitations", { signal: controller.signal });
        if (!controller.signal.aborted) { setInvitations(next); setError(false); }
      } catch { if (!controller.signal.aborted) setError(true); }
      finally { running = false; }
    }
    void update();
    const interval = setInterval(() => void update(), 15000);
    window.addEventListener("online", update);
    return () => { controller.abort(); clearInterval(interval); window.removeEventListener("online", update); };
  }, []);
  const visible = invitations.filter(invite => !hidden.includes(`${invite.tableId}:${invite.expiresAt}`));
  return <div className={compact ? "account-presence-compact" : "account-presence"}>
    {error ? <p role="status" className="form-error">Koneksi akun terputus. Menghubungkan kembali…</p> : null}
    {compact ? null : visible.map(invite => <div className="invite-notice" key={`${invite.tableId}:${invite.sender.id}`}><ProfileAvatar avatar={invite.sender.avatar} online={invite.sender.online} compact /><span><strong>{invite.sender.displayName}</strong> mengundangmu bermain.</span><Link href={`/lobby?invite=${encodeURIComponent(invite.inviteCode)}`}>Lihat meja</Link><button type="button" aria-label="Tutup undangan" onClick={() => setHidden(values => [...values, `${invite.tableId}:${invite.expiresAt}`])}>×</button></div>)}
  </div>;
}
