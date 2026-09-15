"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";
import { accountRequest, type Profile } from "./account-types";
import { createRequestId } from "./request-id";
import { SocialUsers } from "./social-users";

type MatchView = components["schemas"]["MatchView"];
const rooms = ["OPEN", "CLOSED"] as const;
const seats = ["N", "E", "S", "W"] as const;
const statusLabels = { WAITING: "Menunggu pemain siap", ACTIVE: "Sedang dimainkan", COMPLETE: "Selesai", CANCELLED: "Dibatalkan" };

export function MatchLobby({ profile }: { profile: Profile }) {
  const router = useRouter();
  const [matches, setMatches] = useState<MatchView[]>([]);
  const [listError, setListError] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [creating, setCreating] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [boardCount, setBoardCount] = useState(8);
  const [roster, setRoster] = useState<(Profile | null)[]>([profile, null, null, null, null, null, null, null]);
  const [selectedSlot, setSelectedSlot] = useState(1);
  const request = useRef({ body: "", id: "" });
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    async function load() {
      if (running) return;
      running = true;
      try {
        const result = await accountRequest<MatchView[]>("/matches", { signal: controller.signal });
        if (!controller.signal.aborted) { setMatches(result); setListError(""); setLoaded(true); }
      } catch (error) { if (!controller.signal.aborted) { setListError(error instanceof Error ? error.message : "Koneksi terputus."); setLoaded(true); } }
      finally { running = false; }
    }
    void load();
    const interval = setInterval(() => void load(), 5000);
    return () => { controller.abort(); clearInterval(interval); };
  }, []);

  async function create() {
    setPending(true); setError("");
    const assignments = roster.map((player, _index) => ({ userId: player!.id, room: rooms[Math.floor(_index / 4)]!, seat: seats[_index % 4]! }));
    const body = JSON.stringify({ boardCount, assignments });
    if (request.current.body !== body) request.current = { body, id: createRequestId() };
    try {
      const result = await accountRequest<MatchView>("/matches", { method: "POST", body: JSON.stringify({ boardCount, assignments, requestId: request.current.id }) });
      router.push(`/match/${result.id}`);
    } catch (error) { setError(error instanceof Error ? error.message : "Match gagal dibuat. Coba lagi."); }
    finally { setPending(false); }
  }

  return <>
    <section className="match-section" aria-labelledby="your-matches"><h2 id="your-matches">Match kamu</h2>
      {listError ? <p role="alert" className="form-error">{listError} Memeriksa kembali secara otomatis.</p> : null}
      {!loaded ? <p role="status">Memuat match…</p> : matches.length === 0 && !listError ? <p>Belum ada undangan atau match. Buat match untuk mengajak tujuh pemain.</p> : null}
      <ul className="match-list">{matches.map(match => <li key={match.id}><Link href={`/match/${match.id}`}><strong>Tim {match.team} · {match.room} · {match.seat}</strong><span>{statusLabels[match.status]} · {match.boardCount} board</span><span>{match.status === "WAITING" ? `${match.readyCount}/8 siap` : match.status === "COMPLETE" ? `Tim A ${match.teamAIMP} IMP` : `${match.openCompleted}/${match.boardCount} open · ${match.closedCompleted}/${match.boardCount} closed`}</span></Link></li>)}</ul>
    </section>
    <section className="match-section" aria-labelledby="create-match"><h2 id="create-match">Buat match</h2>
      {!creating ? <button className="primary-button" type="button" onClick={() => setCreating(true)}>Susun pemain</button> : <>
        <p>Tim A: NS di Open, EW di Closed. Tim B: EW di Open, NS di Closed. Semua pemain menyatakan siap sebelum dimulai.</p>
        <label className="match-board-count">Jumlah board<input type="number" min={1} max={32} value={boardCount} disabled={pending} onChange={event => setBoardCount(event.target.valueAsNumber)} /></label>
        <div className="match-rooms">{rooms.map((room, _roomIndex) => <fieldset key={room} disabled={pending}><legend>{room === "OPEN" ? "Open room" : "Closed room"}</legend>{seats.map((seat, _seatIndex) => {
          const slot = _roomIndex * 4 + _seatIndex;
          const player = roster[slot];
          const team = (_seatIndex % 2 === 0) === (_roomIndex === 0) ? "A" : "B";
          return <button className="match-seat" type="button" key={seat} disabled={slot === 0} aria-pressed={selectedSlot === slot} aria-label={`${room} ${seat} Tim ${team}`} onClick={() => setSelectedSlot(slot)}><span>{seat} · Tim {team}</span><strong>{player?.displayName ?? "Pilih pemain"}</strong><small>{player ? `@${player.username}` : "Belum diisi"}</small></button>;
        })}</fieldset>)}</div>
        <section aria-label="Pilih pemain untuk kursi"><h3>Pilih untuk {rooms[Math.floor(selectedSlot / 4)]} · {seats[selectedSlot % 4]}</h3>{roster[selectedSlot] ? <button type="button" disabled={pending} onClick={() => setRoster(previous => previous.map((player, _index) => _index === selectedSlot ? null : player))}>Kosongkan kursi ini</button> : null}<SocialUsers viewerId={profile.id} selection={{ selectedIds: roster.flatMap(player => player ? [player.id] : []), onSelect: player => {
          if (pending) return;
          const next = roster.map((previous, _index) => _index === selectedSlot ? player : previous);
          setRoster(next);
          const empty = next.findIndex(candidate => candidate === null);
          if (empty >= 0) setSelectedSlot(empty);
        } }} /></section>
        {error ? <p role="alert" className="form-error">{error}</p> : null}
        <div className="match-actions"><button className="primary-button" type="button" disabled={pending || roster.some(player => player === null) || !Number.isInteger(boardCount) || boardCount < 1 || boardCount > 32} onClick={() => void create()}>{pending ? "Membuat…" : "Buat dan undang 8 pemain"}</button><button type="button" disabled={pending} onClick={() => setCreating(false)}>Tutup pengaturan</button></div>
      </>}
    </section>
  </>;
}

export function MatchProgress({ matchId }: { matchId: string }) {
  const [match, setMatch] = useState<MatchView | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const generation = useRef(0);
  const mutating = useRef(false);
  useEffect(() => {
    const controller = new AbortController();
    let running = false;
    async function load() {
      if (running || mutating.current) return;
      running = true;
      const version = generation.current;
      try {
        const result = await accountRequest<MatchView>(`/matches/${matchId}`, { signal: controller.signal });
        if (!controller.signal.aborted && version === generation.current) { setMatch(result); setError(""); }
      } catch (error) { if (!controller.signal.aborted && version === generation.current) setError(error instanceof Error ? error.message : "Koneksi terputus."); }
      finally { running = false; }
    }
    void load();
    const interval = setInterval(() => void load(), 3000);
    return () => { controller.abort(); clearInterval(interval); };
  }, [matchId, refresh]);

  async function act(action: "ready" | "start" | "cancel") {
    if (!match || mutating.current) return;
    mutating.current = true; generation.current += 1;
    setPending(true); setError("");
    try {
      const body = action === "ready" ? { requestId: createRequestId(), expectedTableRevision: match.tableRevision, ready: !match.ready } : { expectedRevision: match.revision };
      const result = await accountRequest<MatchView>(`/matches/${matchId}/${action}`, { method: "POST", body: JSON.stringify(body) });
      setMatch(result); setConfirmCancel(false);
    } catch (error) { setError(error instanceof Error ? error.message : "Perubahan gagal. Coba lagi."); }
    finally { mutating.current = false; setPending(false); }
  }

  return <>
    {error ? <div role="alert"><p className="form-error">{error}</p><button type="button" disabled={pending} onClick={() => setRefresh(value => value + 1)}>Perbarui status</button></div> : null}
    {!match ? !error ? <p role="status">Memuat match…</p> : null : <>
      <p className="eyebrow">Tim {match.team} · {match.room} · Kursi {match.seat}</p>
      <h2>{statusLabels[match.status]}</h2>
      {match.syncPending ? <p role="status">Match tersimpan. Buka kembali room untuk menyelaraskan koneksi.</p> : null}
      {match.status === "WAITING" ? <section className="match-section"><p>{match.readyCount}/8 pemain siap · {match.boardCount} board</p><p>Kursi tetap selama match. Dengan siap, kamu menyetujui susunan tim dan jumlah board.</p><div className="match-actions"><button type="button" className="primary-button" disabled={pending} onClick={() => void act("ready")}>{match.ready ? "Batalkan siap" : "Saya siap"}</button>{match.isOwner ? <button type="button" disabled={pending || !match.canStart} onClick={() => void act("start")}>Mulai match</button> : <span>Menunggu pemilik memulai setelah semua siap.</span>}</div></section> : null}
      {match.status === "ACTIVE" || match.status === "COMPLETE" ? <section className="match-section"><h3>Progres board final</h3><div className="match-progress"><label>Open · {match.openCompleted}/{match.boardCount}<progress value={match.openCompleted} max={match.boardCount} /></label><label>Closed · {match.closedCompleted}/{match.boardCount}<progress value={match.closedCompleted} max={match.boardCount} /></label></div>{match.status === "ACTIVE" ? <p>Setiap room bergerak mandiri. Pada board terakhir, pemilik room memilih “Selesaikan room” di menu meja. Hasil perbandingan terbuka setelah kedua room selesai.</p> : null}</section> : null}
      {match.status !== "CANCELLED" ? <div className="match-actions"><Link className="primary-button" href={`/table/${match.tableId}`}>Masuk room {match.room}</Link></div> : <p>Match ini dibatalkan. Buat match baru untuk mengganti susunan pemain.</p>}
      {match.status === "COMPLETE" ? <section className="match-section"><h2>Hasil akhir · IMP</h2><p className="match-total">Tim A <strong>{match.teamAIMP}</strong> · Tim B <strong>{-match.teamAIMP || 0}</strong></p><p>{match.teamAIMP === 0 ? "Seri." : `Tim ${match.teamAIMP > 0 ? "A" : "B"} menang dengan selisih ${Math.abs(match.teamAIMP)} IMP.`}</p><div className="match-score-scroll"><table className="match-scores"><caption>Perbandingan poin NS dan IMP Tim A</caption><thead><tr><th>Board</th><th>Open NS</th><th>Closed NS</th><th>IMP A</th></tr></thead><tbody>{match.comparisons?.map((comparison, _index) => <tr key={comparison.boardId}><th scope="row">{_index + 1}</th><td>{comparison.openScoreNS}</td><td>{comparison.closedScoreNS}</td><td>{comparison.teamAIMP}</td></tr>)}</tbody></table></div></section> : null}
      {match.canCancel ? <section className="match-section">{confirmCancel ? <><p>Batalkan match untuk semua delapan pemain? Kedua room akan ditutup.</p><div className="match-actions"><button type="button" disabled={pending} onClick={() => void act("cancel")}>Ya, batalkan match</button><button type="button" disabled={pending} onClick={() => setConfirmCancel(false)}>Kembali</button></div></> : <button type="button" disabled={pending} onClick={() => setConfirmCancel(true)}>Batalkan match</button>}</section> : null}
      {pending ? <p role="status">Menyimpan…</p> : null}
    </>}
  </>;
}
