"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";
import { accountRequest } from "./account-types";
import { BoardReplayModal } from "./table/board-replay-modal";
import { compactContractLabel } from "./table/gameplay-presentation";
import type { LiveTableProjection } from "./table-state";
import { useTableSession } from "./use-table-session";
import { useWorkspacePresentation } from "./workspace-state";

type HistoryBoard = components["schemas"]["HistoryBoard"];
type ScoreSheetEntry = LiveTableProjection["scoreSheet"][number];
type HistoryParticipant = components["schemas"]["HistoryParticipant"];
type HistoryPair = components["schemas"]["HistoryPair"];

const historyDateFormatter = new Intl.DateTimeFormat("id-ID", {
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function HistoryIcon() {
  return <svg className="history-action-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 9 6-9 6z" /></svg>;
}

function compactBoardResult(board: HistoryBoard) {
  if (board.result.passedOut || board.result.contract === undefined) return "Passed out";
  const contract = board.result.contract as NonNullable<ScoreSheetEntry["result"]["contract"]>;
  const difference = board.result.tricksDeclarer - (6 + contract.level);
  const result = difference === 0 ? "=" : difference > 0 ? `+${difference}` : String(difference);
  return `${compactContractLabel(contract)}${result}`;
}

function formatBoardDate(value: string) {
  return historyDateFormatter.format(new Date(value));
}

function searchableBoardValues(board: HistoryBoard) {
  const date = new Date(board.completedAt);
  const isoDate = date.toISOString().slice(0, 10);
  const localDate = historyDateFormatter.format(date).slice(0, 10);
  return [board.label ?? "", compactBoardResult(board), formatBoardDate(board.completedAt), isoDate, localDate].map((value) => value.toLocaleLowerCase());
}

function boardMatchesSearch(board: HistoryBoard, search: string) {
  const normalizedSearch = search.trim().toLocaleLowerCase();
  if (normalizedSearch === "") return true;
  const compactSearch = normalizedSearch.replace(/[♣♦♥♠]/g, (suit) => ({ "♣": "c", "♦": "d", "♥": "h", "♠": "s" })[suit] ?? suit).replace(/\s+/g, "");
  return searchableBoardValues(board).some((value) => value.includes(normalizedSearch) || value.replace(/[♣♦♥♠]/g, (suit) => ({ "♣": "c", "♦": "d", "♥": "h", "♠": "s" })[suit] ?? suit).replace(/\s+/g, "").includes(compactSearch));
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

function HistoryBoardRow({ board, selected, savingLabel, onSelect, onSaveLabel }: { board: HistoryBoard; selected: boolean; savingLabel: boolean; onSelect: (boardId: string) => void; onSaveLabel: (boardId: string, label: string) => void }) {
  const labelInputRef = useRef<HTMLInputElement | null>(null);

  function saveDraftLabel() {
    if (savingLabel) return;
    const nextLabel = labelInputRef.current?.value.trim() ?? "";
    if (nextLabel !== (board.label ?? "")) onSaveLabel(board.boardId, nextLabel);
  }

  return (
    <li className="history-board-list-item">
      <div className="history-board-row" data-selected={selected}>
        <button className="history-board-select" type="button" aria-pressed={selected} onClick={() => onSelect(board.boardId)}>
          <strong className="history-board-result">{compactBoardResult(board)}</strong>
          <time className="history-board-time" dateTime={board.completedAt}>{formatBoardDate(board.completedAt)}</time>
        </button>
        <label className="history-board-label">
          <span>Label belajar</span>
          <input ref={labelInputRef} type="text" defaultValue={board.label ?? ""} maxLength={80} placeholder="Tambah label" aria-label={`Label belajar board ${board.boardNumber}`} disabled={savingLabel} onBlur={saveDraftLabel} onKeyDown={(event) => { if (event.key === "Enter") event.currentTarget.blur(); }} />
        </label>
      </div>
    </li>
  );
}

function HistoryBoardList({ boards, selectedBoardId, loadingMore, hasMore, savingLabelId, onSelect, onLoadMore, onSaveLabel }: { boards: HistoryBoard[]; selectedBoardId: string | null; loadingMore: boolean; hasMore: boolean; savingLabelId: string | null; onSelect: (boardId: string) => void; onLoadMore: () => void; onSaveLabel: (boardId: string, label: string) => void }) {
  const sentinelRef = useRef<HTMLLIElement | null>(null);
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (sentinel === null || !hasMore) return;
    const observer = new IntersectionObserver((entries) => { if (entries[0]?.isIntersecting) onLoadMore(); }, { rootMargin: "480px 0px" });
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, onLoadMore]);

  return <ol className="history-board-list" aria-label="History board" aria-busy={loadingMore}>
    {boards.map((board) => <HistoryBoardRow key={board.boardId} board={board} selected={selectedBoardId === board.boardId} savingLabel={savingLabelId === board.boardId} onSelect={onSelect} onSaveLabel={onSaveLabel} />)}
    {hasMore ? <li ref={sentinelRef} className="history-board-sentinel" aria-hidden="true">{loadingMore ? "Memuat board berikutnya…" : ""}</li> : null}
  </ol>;
}

function HistoryBoardActions({ board, onOpenReplay }: { board: HistoryBoard | null; onOpenReplay: (board: HistoryBoard, trigger: HTMLButtonElement) => void }) {
  if (board === null) return null;
  return <div className="history-board-actions">{board.replayAllowed ? <button className="history-replay-button" type="button" onClick={(event) => onOpenReplay(board, event.currentTarget)}><HistoryIcon />Pelajari board &amp; buka replay</button> : <p>Replay tersedia setelah Team Match selesai.</p>}</div>;
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
  const [labelOverrides, setLabelOverrides] = useState<Record<string, string | null>>({});
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [savingLabelId, setSavingLabelId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [replayBoard, setReplayBoard] = useState<HistoryBoard | null>(null);
  const replayTriggerRef = useRef<HTMLButtonElement | null>(null);
  const requestRef = useRef<AbortController | null>(null);
  const loadingRef = useRef(false);
  const refreshedMatchRef = useRef<string | null>(null);
  const historyBoardId = presentation?.historyBoardId ?? null;
  const setHistoryBoardId = presentation?.setHistoryBoardId;

  const loadPage = useCallback(async (cursor: string | null, replace: boolean, search: string) => {
    if (loadingRef.current && !replace) return;
    if (replace) requestRef.current?.abort();
    loadingRef.current = true;
    if (replace) setLoading(true); else setLoadingMore(true);
    const controller = new AbortController();
    requestRef.current = controller;
    try {
      const query = new URLSearchParams({ limit: "16" });
      if (cursor !== null) query.set("cursor", cursor);
      if (search !== "") query.set("search", search);
      const page = await accountRequest<components["schemas"]["HistoryBoardPage"]>(`/history/boards?${query}`, { signal: controller.signal });
      if (controller.signal.aborted) return;
      setBoards((current) => replace ? page.items : [...current, ...page.items.filter((item) => !current.some((candidate) => candidate.boardId === item.boardId))]);
      setNextCursor(page.nextCursor ?? null);
      setError("");
    } catch (failure) {
      if (!controller.signal.aborted) setError(failure instanceof Error ? failure.message : "Koneksi terputus.");
    } finally {
      if (requestRef.current === controller) {
        loadingRef.current = false;
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => setSearchQuery(searchInput.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const timer = window.setTimeout(() => void loadPage(null, true, searchQuery), 0);
    return () => window.clearTimeout(timer);
  }, [loadPage, searchQuery]);

  useEffect(() => () => requestRef.current?.abort(), []);

  useEffect(() => {
    const matchId = table?.matchId;
    if (matchId === undefined || table?.matchComplete !== true || refreshedMatchRef.current === matchId) return;
    refreshedMatchRef.current = matchId;
    const timer = window.setTimeout(() => void loadPage(null, true, searchQuery), 0);
    return () => window.clearTimeout(timer);
  }, [loadPage, searchQuery, table?.matchComplete, table?.matchId]);

  const localBoards = table?.scoreSheet.map((entry) => ({ boardId: entry.boardId, tableId: table.tableId, boardNumber: entry.boardNumber, result: entry.result, lineup: entry.lineup, viewerSeat: table.viewerSeat ?? "N", completedAt: new Date().toISOString(), replayAllowed: !(table.matchId !== undefined && table.matchComplete !== true) } as HistoryBoard)) ?? [];
  const allBoards: HistoryBoard[] = [...boards, ...localBoards.filter((local) => !boards.some((board) => board.boardId === local.boardId))].map((board): HistoryBoard => {
    if (!Object.prototype.hasOwnProperty.call(labelOverrides, board.boardId)) return board;
    const label = labelOverrides[board.boardId];
    if (label === null || label === undefined) {
      const boardWithoutLabel = { ...board };
      delete boardWithoutLabel.label;
      return boardWithoutLabel;
    }
    return { ...board, label };
  }).sort((first, second) => Date.parse(second.completedAt) - Date.parse(first.completedAt));
  const visibleBoards = allBoards.filter((board) => boardMatchesSearch(board, searchQuery));
  const selectedBoard = visibleBoards.find((board) => board.boardId === historyBoardId) ?? visibleBoards[0] ?? null;

  const saveLabel = useCallback(async (boardId: string, label: string) => {
    setSavingLabelId(boardId);
    try {
      if (label === "") {
        await accountRequest(`/history/boards/${boardId}/label`, { method: "DELETE" });
        setLabelOverrides((current) => ({ ...current, [boardId]: null }));
      } else {
        await accountRequest<components["schemas"]["HistoryBoardLabel"]>(`/history/boards/${boardId}/label`, { method: "PUT", body: JSON.stringify({ label }) });
        setLabelOverrides((current) => ({ ...current, [boardId]: label }));
      }
      setError("");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "Label belum tersimpan.");
    } finally {
      setSavingLabelId(null);
    }
  }, []);

  function openReplay(board: HistoryBoard, trigger: HTMLButtonElement) {
    replayTriggerRef.current = trigger;
    setReplayBoard(board);
  }

  const replayEntry = replayBoard === null ? null : scoreEntryFromHistory(replayBoard);
  const replayTable = replayBoard === null || replayEntry === null ? null : replayTableFor(replayBoard, replayEntry);

  return <div className="history-workspace"><section className="history-board-section" aria-labelledby="board-history-title"><header className="history-section-heading"><div><h1 id="board-history-title">Recent boards</h1><p>Semua board yang pernah kamu mainkan, dari casual, bot, maupun Team Match.</p></div><label className="history-search-field"><span>Cari history</span><input type="search" value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="Tanggal, label, atau contract" aria-label="Cari berdasarkan tanggal, label, atau contract" /></label></header>{error ? <div className="history-load-error" role="alert"><p>{error}</p><button type="button" onClick={() => void loadPage(null, true, searchQuery)}>Coba lagi</button></div> : null}{loading ? <p role="status">Memuat history…</p> : visibleBoards.length === 0 ? <p className="history-empty-copy">Tidak ada board yang cocok.</p> : <><HistoryBoardList boards={visibleBoards} selectedBoardId={selectedBoard?.boardId ?? null} loadingMore={loadingMore} hasMore={nextCursor !== null} savingLabelId={savingLabelId} onSelect={(boardId) => setHistoryBoardId?.(boardId)} onLoadMore={() => void loadPage(nextCursor, false, searchQuery)} onSaveLabel={(boardId, label) => void saveLabel(boardId, label)} /><HistoryBoardActions board={selectedBoard} onOpenReplay={openReplay} /></>}{replayTable === null || replayEntry === null ? null : <BoardReplayModal key={replayBoard?.boardId} table={replayTable} entry={replayEntry} loadBoardReplay={session.loadBoardReplay} loadPositionAnalysis={session.loadPositionAnalysis} onClose={() => { setReplayBoard(null); requestAnimationFrame(() => replayTriggerRef.current?.focus()); }} />}</section></div>;
}
