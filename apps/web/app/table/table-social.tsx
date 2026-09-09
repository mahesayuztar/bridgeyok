"use client";

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { accountRequest, type Profile } from "../account-types";
import { SocialUsers } from "../social-users";
import { useDialogDrag } from "./use-dialog-drag";

const TableSocialContext = createContext<{ viewerId: string; profiles: Profile[]; error: string; reload: () => void } | null>(null);

export function useTableSocial() { return useContext(TableSocialContext); }

export function TableSocialProvider({ tableId, viewerId, children }: { tableId: string; viewerId: string; children: ReactNode }) {
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    async function update() {
      if (running) return;
      running = true;
      try {
        const profiles = await accountRequest<Profile[]>(`/tables/${tableId}/participants`, { signal: controller.signal });
        if (!controller.signal.aborted) { setProfiles(profiles); setError(""); }
      } catch { if (!controller.signal.aborted) setError("Profil pemain belum terhubung."); }
      finally { running = false; }
    }
    void update();
    const interval = setInterval(() => void update(), 15000);
    return () => { controller.abort(); clearInterval(interval); };
  }, [tableId, revision]);
  return <TableSocialContext value={{ viewerId, profiles, error, reload: () => setRevision(value => value + 1) }}>{children}</TableSocialContext>;
}

export function TableInvite({ tableId, inviteCode, disabled }: { tableId: string; inviteCode: string | null; disabled: boolean }) {
  const social = useTableSocial();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const drag = useDialogDrag();
  if (!social) return null;
  return <><button type="button" ref={triggerRef} disabled={disabled || !inviteCode} onClick={() => { setOpen(true); dialogRef.current?.showModal(); }}>Invite player</button>
    <dialog ref={dialogRef} className="social-dialog" onClose={() => { setOpen(false); triggerRef.current?.focus(); }} aria-labelledby="invite-player-title"><header {...drag}><h2 id="invite-player-title">Invite player</h2><button type="button" aria-label="Tutup invite" onClick={() => dialogRef.current?.close()}>×</button></header><p>Undangan tidak mengambil kursi. Pemain memilih kursinya setelah bergabung.</p>{open && inviteCode ? <SocialUsers viewerId={social.viewerId} table={{ id: tableId, inviteCode, disabled }} /> : null}</dialog>
  </>;
}
