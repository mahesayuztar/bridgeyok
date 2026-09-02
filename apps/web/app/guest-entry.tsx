"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import IssueNotice from "./issue-notice";
import { useTableSession } from "./use-table-session";

export default function GuestEntry({ initialInviteCode = "" }: { initialInviteCode?: string }) {
  const router = useRouter();
  const session = useTableSession({ connectOnRestore: false });
  const [nickname, setNickname] = useState("");

  useEffect(() => {
    if (session.initializing) {
      return;
    }
    if (session.recoveryState === "TABLE_ACTIVE" && session.tableState.activeTableId !== null) {
      router.replace(`/table/${session.tableState.activeTableId}`);
    } else if (session.recoveryState === "TABLE_EXPIRED") {
      router.replace("/lobby");
    }
  }, [router, session.initializing, session.recoveryState, session.tableState.activeTableId]);

  async function submitIdentity(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (nickname.trim().length === 0) {
      return;
    }
    const created = await session.createIdentity(nickname);
    if (created) {
      const query = initialInviteCode === "" ? "" : `?invite=${encodeURIComponent(initialInviteCode)}`;
      router.replace(`/lobby${query}`);
    }
  }

  return (
    <section className="identity-entry" id="mulai" aria-labelledby="identity-title">
      <div>
        <p className="eyebrow">Mulai tanpa akun</p>
        <h2 id="identity-title">Namamu cukup untuk duduk bermain.</h2>
        <p>Sesi tamu tersimpan di perangkat ini. Tidak ada kata sandi dan tidak ada pembayaran.</p>
        {!session.initializing && session.nickname !== null ? (
          <Link className="text-link" href={initialInviteCode === "" ? "/lobby" : `/lobby?invite=${encodeURIComponent(initialInviteCode)}`}>
            Lanjut ke lobby <span aria-hidden="true">→</span>
          </Link>
        ) : null}
      </div>
      {session.initializing ? (
        <div className="identity-loading" role="status" aria-live="polite">
          <span className="loading-spinner" aria-hidden="true" />
          <div><strong>Memeriksa sesi tersimpan…</strong><p>Ini hanya memerlukan beberapa saat.</p></div>
        </div>
      ) : (
        <form onSubmit={submitIdentity} aria-busy={session.busy}>
          <label htmlFor="nickname">Nama di meja</label>
          <input id="nickname" name="nickname" autoComplete="nickname" maxLength={128} required disabled={session.busy} value={nickname} onChange={(event) => setNickname(event.target.value)} />
          <button className="primary-button loading-button" type="submit" disabled={session.busy}>
            {session.busy ? <><span className="loading-spinner" aria-hidden="true" />Menyiapkan sesi…</> : "Masuk sebagai tamu"}
          </button>
        </form>
      )}
      {session.tableState.issue === null ? null : <IssueNotice issue={session.tableState.issue} onDismiss={session.dismissIssue} />}
    </section>
  );
}
