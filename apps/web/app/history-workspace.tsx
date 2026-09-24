"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";
import { accountRequest } from "./account-types";
import { BoardReplayModal } from "./table/board-replay-modal";
import { contractLabel } from "./table/gameplay-presentation";
import { boardResultLabel, type LiveTableProjection, type Seat } from "./table-state";
import { useTableSession } from "./use-table-session";
import { useWorkspacePresentation } from "./workspace-state";

type HistoryBoard = components["schemas"]["HistoryBoard"];
type ScoreSheetEntry = LiveTableProjection["scoreSheet"][number];
type HistoryParticipant = components["schemas"]["HistoryParticipant"];
type HistoryPair = components["schemas"]["HistoryPair"];

const vulnerabilityLabels = {
  NONE: "Tidak vul",
  NS: "NS vul",
  EW: "EW vul",
  BOTH: "Kedua sisi vul",
};

function signedScore(score: number) {
  return score > 0 ? `+${score}` : String(score);
}

function boardNumberLabel(boardNumber: number) {
  return String(boardNumber).padStart(2, "0");
}

function viewerPartnership(viewerSeat: Seat) {
  return viewerSeat === "E" || viewerSeat === "W" ? "EW" : "NS";
}

function scoreContext(board: HistoryBoard) {
  const perspective = viewerPartnership(board.viewerSeat);
  const perspectiveScore = perspective === "EW" ? -board.result.scoreNS : board.result.scoreNS;
  return {
    perspective,
    perspectiveScore,
    resultLabel: board.result.passedOut ? "Passed out" : board.result.contract === undefined ? "Result tidak tersedia" : boardResultLabel(board.result as ScoreSheetEntry["result"]),
  };
}

function HistoryIcon({ name }: { name: "arrow-left" | "play" }) {
  if (name === "arrow-left") {
    return <svg className="history-action-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M11 6l-6 6 6 6" /></svg>;
  }
  return <svg className="history-action-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 9 6-9 6z" /></svg>;
}

function participantFromHistory(participant: HistoryParticipant) {
  return { id: participant.id, nickname: participant.nickname, isBot: participant.isBot };
}

function pairFromHistory(pair: HistoryPair) {
  const [first, second] = pair.members;
  if (first === undefined || second === undefined) return null;
  return {
    id: pair.id,
    members: [participantFromHistory(first), participantFromHistory(second)] as ScoreSheetEntry["lineup"]["northSouth"]["members"],
  };
}

function scoreEntryFromHistory(board: HistoryBoard): ScoreSheetEntry | null {
  const seats = {} as ScoreSheetEntry["lineup"]["seats"];
  for (const seat of ["N", "E", "S", "W"] as const) {
    const participant = board.lineup.seats[seat];
    if (participant === undefined) return null;
    seats[seat] = participantFromHistory(participant);
  }
  const northSouth = pairFromHistory(board.lineup.northSouth);
  const eastWest = pairFromHistory(board.lineup.eastWest);
  if (northSouth === null || eastWest === null) return null;
  return {
    boardId: board.boardId,
    boardNumber: board.boardNumber,
    result: board.result as ScoreSheetEntry["result"],
    lineup: { seats, northSouth, eastWest },
  };
}

function lineupLabel(board: HistoryBoard) {
  const seats = board.lineup.seats;
  const northSouth = [seats.N?.nickname, seats.S?.nickname].filter(Boolean).join(" / ");
  const eastWest = [seats.E?.nickname, seats.W?.nickname].filter(Boolean).join(" / ");
  return `NS ${northSouth} · EW ${eastWest}`;
}

function ContractValue({ board, showDeclarer }: { board: HistoryBoard; showDeclarer: boolean }) {
  if (board.result.passedOut) return <strong>Passed out</strong>;
  if (board.result.contract === undefined) return <strong>Kontrak tidak tersedia</strong>;
  return <span className="history-contract" data-strain={board.result.contract.strain}>{contractLabel(board.result.contract as ScoreSheetEntry["result"]["contract"])}{showDeclarer ? ` · ${board.result.contract.declarer}` : ""}</span>;
}

function ScorePair({ scoreNS }: { scoreNS: number }) {
  const scoreEW = -scoreNS;
  return (
    <div className="history-score-pair" aria-label={`NS ${signedScore(scoreNS)}, EW ${signedScore(scoreEW)}`}>
      <div data-score-sign={scoreNS === 0 ? "zero" : scoreNS > 0 ? "positive" : "negative"}><span>NS</span><strong>{signedScore(scoreNS)}</strong></div>
      <div data-score-sign={scoreEW === 0 ? "zero" : scoreEW > 0 ? "positive" : "negative"}><span>EW</span><strong>{signedScore(scoreEW)}</strong></div>
    </div>
  );
}

function SessionSummary({ table }: { table: LiveTableProjection | null }) {
  if (table === null) {
    return <section className="history-session-summary history-session-empty" aria-labelledby="history-session-title"><div><span className="history-kicker">Session</span><h2 id="history-session-title">Tidak ada meja aktif</h2><p>History board tetap tersedia dari sesi yang pernah kamu ikuti.</p></div></section>;
  }
  const scoreNS = table.scoreSheet.reduce((total, entry) => total + entry.result.scoreNS, 0);
  return (
    <section className="history-session-summary" aria-labelledby="history-session-title">
      <div className="history-summary-heading"><span className="history-kicker">Session</span><h2 id="history-session-title">Sesi aktif</h2></div>
      <div className="history-summary-metrics" aria-label="Ringkasan sesi"><div><span>Boards</span><strong>{table.scoreSheet.length}</strong></div><div data-score-sign={scoreNS === 0 ? "zero" : scoreNS > 0 ? "positive" : "negative"}><span>NS</span><strong>{signedScore(scoreNS)}</strong></div><div data-score-sign={scoreNS === 0 ? "zero" : scoreNS < 0 ? "positive" : "negative"}><span>EW</span><strong>{signedScore(-scoreNS)}</strong></div></div>
      <Link className="history-back-link" href={`/table/${table.tableId}`}><HistoryIcon name="arrow-left" />Kembali ke meja</Link>
    </section>
  );
}

function HistoryBoardList({ boards, selectedBoardId, loadingMore, hasMore, onSelect, onLoadMore }: { boards: HistoryBoard[]; selectedBoardId: string | null; loadingMore: boolean; hasMore: boolean; onSelect: (boardId: string) => void; onLoadMore: () => void }) {
  const sentinelRef = useRef<HTMLLIElement | null>(null);
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (sentinel === null || !hasMore) return;
    const observer = new IntersectionObserver((entries) => { if (entries[0]?.isIntersecting) onLoadMore(); }, { rootMargin: "480px 0px" });
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, onLoadMore]);

  return (
    <div className="history-board-navigation">
      <div className="history-section-label"><span className="history-kicker">Recent boards</span><h3>Semua hasil</h3></div>
      {boards.length === 0 && !loadingMore ? <p className="history-empty-copy">Belum ada board selesai.</p> : <ol className="history-board-list" aria-label="History board" aria-busy={loadingMore}>
        {boards.map((board) => {
          const context = scoreContext(board);
          return <li key={board.boardId}><button className="history-board-row" type="button" aria-pressed={selectedBoardId === board.boardId} onClick={() => onSelect(board.boardId)}><span className="history-board-number">Board {boardNumberLabel(board.boardNumber)}</span><strong className="history-board-contract"><ContractValue board={board} showDeclarer /></strong><strong className="history-board-result">{context.resultLabel}</strong><span className="history-board-score">{context.perspective} {signedScore(context.perspectiveScore)}</span><span className="history-board-meta">{vulnerabilityLabels[board.result.vulnerability]} · {lineupLabel(board)}</span>{board.teamAIMP === undefined ? null : <span className="history-board-meta">{signedScore(board.teamAIMP)} IMP</span>}</button></li>;
        })}
        {hasMore ? <li ref={sentinelRef} className="history-board-sentinel" aria-hidden="true">{loadingMore ? "Memuat board berikutnya…" : ""}</li> : null}
      </ol>}
    </div>
  );
}

function BoardDetail({ board, onOpenReplay }: { board: HistoryBoard | null; onOpenReplay: (board: HistoryBoard, trigger: HTMLButtonElement) => void }) {
  if (board === null) return <div className="history-board-detail history-board-detail-empty"><span className="history-kicker">Selected board</span><h3>Pilih board untuk melihat detail</h3><p>Kontrak, result, score, dan replay akan muncul di sini.</p></div>;
  const context = scoreContext(board);
  return <article className="history-board-detail" aria-labelledby="selected-board-title"><header className="history-detail-heading"><div><span className="history-kicker">Board {boardNumberLabel(board.boardNumber)}</span><h3 id="selected-board-title"><ContractValue board={board} showDeclarer={false} /></h3>{board.result.passedOut ? null : <p className="history-detail-declarer">Declarer {board.result.contract?.declarer}</p>}</div><span className="history-vulnerability">{vulnerabilityLabels[board.result.vulnerability]}</span></header><p className="history-detail-players">{lineupLabel(board)}</p><div className="history-outcome"><div className="history-result-block"><span className="history-label">Result</span><strong className="history-result">{context.resultLabel}</strong><span className="history-perspective">Perspektif: {context.perspective} {signedScore(context.perspectiveScore)}</span>{board.teamAIMP === undefined ? null : <span className="history-perspective">Team Match: {signedScore(board.teamAIMP)} IMP</span>}</div><ScorePair scoreNS={board.result.scoreNS} /></div><footer className="history-detail-actions">{board.replayAllowed ? <button className="history-replay-button" type="button" onClick={(event) => onOpenReplay(board, event.currentTarget)}><HistoryIcon name="play" />Buka replay &amp; analysis</button> : <p>Replay dan analysis tersedia setelah Team Match selesai.</p>}</footer></article>;
}

function replayTableFor(board: HistoryBoard, entry: ScoreSheetEntry): LiveTableProjection {
  const seats: LiveTableProjection["seats"] = {};
  const participants: LiveTableProjection["participants"] = [];
  for (const seat of ["N", "E", "S", "W"] as const) {
    const participant = entry.lineup.seats[seat];
    seats[seat] = { participantId: participant.id, ready: true, controllerEpoch: 1, isBot: participant.isBot };
    participants.push({ id: participant.id, nickname: participant.nickname, role: "PARTICIPANT", isBot: participant.isBot });
  }
  return { tableId: board.tableId, state: "FINISHED", locked: false, revision: 0, lastSeq: 0, boardId: board.boardId, boardNumber: board.boardNumber, scoreSheet: [entry], pairScoreTotals: [], viewerParticipantId: entry.lineup.seats[board.viewerSeat].id, viewerRole: "PARTICIPANT", viewerSeat: board.viewerSeat, participants, seats, canRequestUndo: false, ...(board.matchId === undefined ? {} : { matchId: board.matchId, matchBoardCount: 0, matchComplete: board.matchStatus === "COMPLETE" }) };
}

export function HistoryWorkspace() {
  const session = useTableSession();
  const table = session.projectedTable;
  const presentation = useWorkspacePresentation();
  const [boards, setBoards] = useState<HistoryBoard[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [replayBoard, setReplayBoard] = useState<HistoryBoard | null>(null);
  const replayTriggerRef = useRef<HTMLButtonElement | null>(null);
  const requestRef = useRef<AbortController | null>(null);
  const loadingRef = useRef(false);
  const refreshedMatchRef = useRef<string | null>(null);
  const historyBoardId = presentation?.historyBoardId ?? null;
  const setHistoryBoardId = presentation?.setHistoryBoardId;

  const loadPage = useCallback(async (cursor: string | null, replace: boolean) => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    if (replace) setLoading(true); else setLoadingMore(true);
    const controller = new AbortController();
    requestRef.current?.abort();
    requestRef.current = controller;
    try {
      const query = new URLSearchParams({ limit: "16" });
      if (cursor !== null) query.set("cursor", cursor);
      const page = await accountRequest<components["schemas"]["HistoryBoardPage"]>(`/history/boards?${query}`, { signal: controller.signal });
      if (controller.signal.aborted) return;
      setBoards((current) => replace ? page.items : [...current, ...page.items.filter((item) => !current.some((candidate) => candidate.boardId === item.boardId))]);
      setNextCursor(page.nextCursor ?? null);
      setError("");
    } catch (failure) {
      if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : "Koneksi terputus.");
    } finally {
      if (!controller.signal.aborted) { loadingRef.current = false; setLoading(false); setLoadingMore(false); }
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadPage(null, true), 0);
    return () => { window.clearTimeout(timer); requestRef.current?.abort(); };
  }, [loadPage]);

  useEffect(() => {
    const matchId = table?.matchId;
    if (matchId === undefined || table?.matchComplete !== true || refreshedMatchRef.current === matchId) return;
    refreshedMatchRef.current = matchId;
    const timer = window.setTimeout(() => void loadPage(null, true), 0);
    return () => window.clearTimeout(timer);
  }, [loadPage, table?.matchComplete, table?.matchId]);

  const localBoards = table?.scoreSheet.map((entry) => ({ boardId: entry.boardId, tableId: table.tableId, boardNumber: entry.boardNumber, result: entry.result, lineup: entry.lineup, viewerSeat: table.viewerSeat ?? "N", completedAt: new Date().toISOString(), replayAllowed: !(table.matchId !== undefined && table.matchComplete !== true) } as HistoryBoard)) ?? [];
  const allBoards = [...boards, ...localBoards.filter((local) => !boards.some((board) => board.boardId === local.boardId))].sort((first, second) => Date.parse(second.completedAt) - Date.parse(first.completedAt));
  const selectedBoard = historyBoardId === null ? allBoards[0] ?? null : allBoards.find((board) => board.boardId === historyBoardId) ?? null;

  function openReplay(board: HistoryBoard, trigger: HTMLButtonElement) { replayTriggerRef.current = trigger; setReplayBoard(board); }

  const replayEntry = replayBoard === null ? null : scoreEntryFromHistory(replayBoard);
  const replayTable = replayBoard === null || replayEntry === null ? null : replayTableFor(replayBoard, replayEntry);

  return <div className="history-workspace"><SessionSummary table={table} /><section className="history-board-section" aria-labelledby="board-history-title"><div className="history-section-heading"><div><span className="history-kicker">Board history</span><h2 id="board-history-title">Recent boards</h2><p>Semua board terakhir dari meja casual, bot, dan Team Match tampil dalam satu daftar.</p></div></div>{error ? <div className="history-load-error" role="alert"><p className="form-error">{error}</p><button type="button" onClick={() => void loadPage(null, true)}>Coba lagi</button></div> : null}{loading ? <p role="status">Memuat history…</p> : <div className="history-board-workspace"><HistoryBoardList boards={allBoards} selectedBoardId={selectedBoard?.boardId ?? null} loadingMore={loadingMore} hasMore={nextCursor !== null} onSelect={(boardId) => setHistoryBoardId?.(boardId)} onLoadMore={() => void loadPage(nextCursor, false)} /><BoardDetail board={selectedBoard} onOpenReplay={openReplay} /></div>}{replayTable === null || replayEntry === null ? null : <BoardReplayModal key={replayBoard?.boardId} table={replayTable} entry={replayEntry} loadBoardReplay={session.loadBoardReplay} loadPositionAnalysis={session.loadPositionAnalysis} onClose={() => { setReplayBoard(null); requestAnimationFrame(() => replayTriggerRef.current?.focus()); }} />}</section></div>;
}
