"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { useTableSession } from "./use-table-session";

export default function GuestEntry({ initialInviteCode = "" }: { initialInviteCode?: string }) {
  const router = useRouter();
  const session = useTableSession({ restoreTable: false });
  const [nickname, setNickname] = useState("");

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
      <form onSubmit={submitIdentity}>
        <label htmlFor="nickname">Nama di meja</label>
        <input id="nickname" name="nickname" autoComplete="nickname" maxLength={128} required value={nickname} onChange={(event) => setNickname(event.target.value)} />
        <button className="primary-button" type="submit" disabled={session.busy || session.initializing}>Masuk sebagai tamu</button>
      </form>
      {session.tableState.message === null ? null : <p className="form-message" role="alert">{session.tableState.message}</p>}
    </section>
  );
}
