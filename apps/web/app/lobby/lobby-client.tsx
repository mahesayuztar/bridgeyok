"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import IssueNotice from "../issue-notice";
import { useTableSession } from "../use-table-session";

export default function LobbyClient({ initialInviteCode = "" }: { initialInviteCode?: string }) {
  const router = useRouter();
  const session = useTableSession();
  const [joinCode, setJoinCode] = useState(initialInviteCode.trim().toUpperCase());
  const [pendingSwitch, setPendingSwitch] = useState<"create" | "join" | null>(null);
  const [logoutError, setLogoutError] = useState(false);

  useEffect(() => {
    if (!session.initializing && session.nickname === null && session.tableState.issue === null) {
      router.replace("/login");
    } else if (initialInviteCode === "" && !session.initializing && session.recoveryState === "TABLE_ACTIVE" && session.tableState.activeTableId !== null) {
      router.replace(`/table/${session.tableState.activeTableId}`);
    }
  }, [initialInviteCode, router, session.initializing, session.nickname, session.recoveryState, session.tableState.activeTableId, session.tableState.issue]);

  async function createTable() {
    if (session.tableState.activeTableId !== null) {
      setPendingSwitch("create");
      return;
    }
    const tableId = await session.createTable();
    if (tableId) router.push(`/table/${tableId}`);
  }

  async function submitJoin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (session.tableState.activeTableId !== null) {
      setPendingSwitch("join");
      return;
    }
    const tableId = await session.joinTable(joinCode);
    if (tableId) router.push(`/table/${tableId}`);
  }

  async function confirmSwitch() {
    const requestedAction = pendingSwitch;
    if (requestedAction === null) return;
    const left = await session.leaveTable();
    if (!left) return;
    setPendingSwitch(null);
    const tableId = requestedAction === "create" ? await session.createTable() : await session.joinTable(joinCode);
    if (tableId) router.push(`/table/${tableId}`);
  }

  async function logout() {
    setLogoutError(false);
    if (await session.logout()) window.location.replace("/login");
    else setLogoutError(true);
  }

  if (session.initializing || session.nickname === null) {
    return <main className="loading-state">{session.tableState.issue ? <IssueNotice issue={session.tableState.issue} onAction={() => window.location.reload()} /> : <p role="status">Memulihkan sesi…</p>}</main>;
  }

  return (
    <div className="app-page">
      <header className="app-navbar">
        <Link className="wordmark" href="/play">BridgeYok</Link>
        <div className="app-identity">
          <span>{session.nickname}</span>
          <button className="quiet-button" type="button" disabled={session.busy} onClick={() => void logout()}>Logout</button>
        </div>
      </header>
      <main className="lobby-page">
        <section className="lobby" aria-labelledby="lobby-title">
          <div className="lobby-heading">
            <div>
              <h1 id="lobby-title">Halo, {session.nickname}</h1>
              <p>Buat meja baru atau gunakan kode undangan dari teman.</p>
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
          {pendingSwitch === null ? null : (
            <section className="lobby-switch" aria-labelledby="lobby-switch-title">
              <h2 id="lobby-switch-title">Tinggalkan meja aktif?</h2>
              <p>{pendingSwitch === "join" ? "Untuk masuk ke meja undangan, kamu harus meninggalkan meja yang sedang aktif lebih dulu." : "Untuk membuat meja baru, kamu harus meninggalkan meja yang sedang aktif lebih dulu."}</p>
              <div className="workspace-switch-actions">
                <button className="primary-button" type="button" disabled={session.busy} onClick={() => void confirmSwitch()}>{session.busy ? "Memeriksa…" : "Tinggalkan dan lanjutkan"}</button>
                <button type="button" disabled={session.busy} onClick={() => setPendingSwitch(null)}>Batal</button>
              </div>
            </section>
          )}
          <div className="lobby-actions">
            <div className="lobby-option">
              <h2>Meja baru</h2>
              <p>Jadilah pemilik meja dan bagikan undangan kepada tiga teman.</p>
              <button className="primary-button" type="button" disabled={session.busy} onClick={() => void createTable()}>Buat meja</button>
            </div>
            <form className="lobby-option" onSubmit={submitJoin}>
              <h2>Masuk meja</h2>
              <label htmlFor="invite-code">Kode undangan</label>
              <input id="invite-code" name="inviteCode" autoCapitalize="characters" autoComplete="off" spellCheck={false} required value={joinCode} onChange={(event) => setJoinCode(event.target.value.toUpperCase())} />
              <button className="secondary-button" type="submit" disabled={session.busy}>Masuk</button>
            </form>
          </div>
          {session.tableState.issue === null ? null : (
            <IssueNotice
              issue={session.tableState.issue}
              onDismiss={session.dismissIssue}
              onAction={(action) => {
                if (action === "editInvite") {
                  document.querySelector<HTMLInputElement>("#invite-code")?.focus();
                } else if (action === "backToLobby") {
                  session.dismissIssue();
                } else if (action === "signInAgain") {
                  void logout();
                } else if (action === "retry" && joinCode.length > 0) {
                  if (session.tableState.activeTableId !== null) void confirmSwitch();
                  else void session.joinTable(joinCode);
                }
              }}
            />
          )}
          {logoutError ? <p className="form-error" role="alert">Logout gagal. Sesi dan meja tetap aktif; coba lagi.</p> : null}
        </section>
      </main>
    </div>
  );
}
