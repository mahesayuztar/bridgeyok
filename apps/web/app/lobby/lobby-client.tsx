"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { useTableSession } from "../use-table-session";

export default function LobbyClient({ initialInviteCode = "" }: { initialInviteCode?: string }) {
  const router = useRouter();
  const session = useTableSession();
  const [joinCode, setJoinCode] = useState(initialInviteCode.trim().toUpperCase());

  useEffect(() => {
    if (!session.initializing && session.nickname === null) {
      router.replace("/");
    }
  }, [router, session.initializing, session.nickname]);

  async function createTable() {
    const tableId = await session.createTable();
    if (tableId !== null) {
      router.push(`/table/${tableId}`);
    }
  }

  async function submitJoin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const tableId = await session.joinTable(joinCode);
    if (tableId !== null) {
      router.push(`/table/${tableId}`);
    }
  }

  async function logout() {
    await session.logout();
    router.replace("/");
  }

  if (session.initializing || session.nickname === null) {
    return <main className="loading-state" role="status"><p>Memulihkan sesi tamu…</p></main>;
  }

  return (
    <div className="app-page">
      <header className="app-navbar">
        <Link className="wordmark" href="/">BridgeYok</Link>
        <div className="app-identity">
          <span>{session.nickname}</span>
          <button className="quiet-button" type="button" disabled={session.busy} onClick={() => void logout()}>Ganti nama</button>
        </div>
      </header>
      <main className="lobby-page">
        <section className="lobby" aria-labelledby="lobby-title">
          <div className="lobby-heading">
            <div>
              <p className="eyebrow">Lobby pribadi</p>
              <h1 id="lobby-title">Halo, {session.nickname}</h1>
              <p>Pilih cara duduk di meja. Kamu dapat membuat undangan baru atau memakai kode dari teman.</p>
            </div>
          </div>
          {session.tableState.table === null ? null : (
            <div className="resume-table">
              <div>
                <span>Meja terakhir</span>
                <strong>{session.tableState.table.state === "WAITING" ? "Menunggu pemain" : `Board ${session.tableState.table.boardNumber}`}</strong>
              </div>
              <Link className="secondary-button link-button" href={`/table/${session.tableState.table.tableId}`}>Lanjutkan meja</Link>
            </div>
          )}
          <div className="lobby-actions">
            <div className="lobby-option">
              <span className="option-number">01</span>
              <h2>Meja baru</h2>
              <p>Jadilah pemilik meja dan bagikan undangan kepada tiga teman.</p>
              <button className="primary-button" type="button" disabled={session.busy} onClick={() => void createTable()}>Buat meja</button>
            </div>
            <form className="lobby-option" onSubmit={submitJoin}>
              <span className="option-number">02</span>
              <h2>Masuk meja</h2>
              <label htmlFor="invite-code">Kode undangan</label>
              <input id="invite-code" name="inviteCode" autoCapitalize="characters" autoComplete="off" spellCheck={false} required value={joinCode} onChange={(event) => setJoinCode(event.target.value.toUpperCase())} />
              <button className="secondary-button" type="submit" disabled={session.busy}>Masuk</button>
            </form>
          </div>
          {session.tableState.message === null ? null : <p className="form-message" role="alert">{session.tableState.message}</p>}
        </section>
      </main>
    </div>
  );
}
