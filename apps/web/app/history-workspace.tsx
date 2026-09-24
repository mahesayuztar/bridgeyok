"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import type { components } from "@bridgeyok/contracts/openapi";
import { accountRequest } from "./account-types";
import { BoardReplayModal } from "./table/board-replay-modal";
import { contractLabel } from "./table/gameplay-presentation";
import { boardResultLabel, type LiveTableProjection } from "./table-state";
import { useTableSession } from "./use-table-session";
import { useWorkspacePresentation } from "./workspace-state";

type MatchView = components["schemas"]["MatchView"];
type ScoreSheetEntry = LiveTableProjection["scoreSheet"][number];
type ReplaySelection = { tableId: string; boardId: string; matchComplete: boolean };
type HistoryIconName = "arrow-left" | "play";

const matchStatusLabels = {
  WAITING: "Menunggu pemain siap",
  ACTIVE: "Sedang dimainkan",
  COMPLETE: "Selesai",
  CANCELLED: "Dibatalkan",
};

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

function viewerPartnership(viewerSeat: LiveTableProjection["viewerSeat"]) {
  return viewerSeat === "E" || viewerSeat === "W" ? "EW" : "NS";
}

function scoreContext(entry: ScoreSheetEntry, viewerSeat: LiveTableProjection["viewerSeat"]) {
  const { result } = entry;
  const perspective = viewerPartnership(viewerSeat);
  const perspectiveScore = perspective === "EW" ? -result.scoreNS : result.scoreNS;
  return {
    perspective,
    perspectiveScore,
    resultLabel: result.passedOut ? "Passed out" : result.contract === undefined ? "Result tidak tersedia" : boardResultLabel(result),
  };
}

function HistoryIcon({ name }: { name: HistoryIconName }) {
  if (name === "arrow-left") {
    return <svg className="history-action-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M11 6l-6 6 6 6" /></svg>;
  }
  return <svg className="history-action-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 9 6-9 6z" /></svg>;
}

function ContractValue({ entry, showDeclarer }: { entry: ScoreSheetEntry; showDeclarer: boolean }) {
  if (entry.result.passedOut) return <strong>Passed out</strong>;
  if (entry.result.contract === undefined) return <strong>Kontrak tidak tersedia</strong>;
  return (
    <span className="history-contract" data-strain={entry.result.contract.strain}>
      {contractLabel(entry.result.contract)}{showDeclarer ? ` · ${entry.result.contract.declarer}` : ""}
    </span>
  );
}

function ScorePair({ scoreNS }: { scoreNS: number }) {
  const scoreEW = -scoreNS;
  return (
    <div className="history-score-pair" aria-label={`NS ${signedScore(scoreNS)}, EW ${signedScore(scoreEW)}`}>
      <div data-score-sign={scoreNS === 0 ? "zero" : scoreNS > 0 ? "positive" : "negative"}>
        <span>NS</span>
        <strong>{signedScore(scoreNS)}</strong>
      </div>
      <div data-score-sign={scoreEW === 0 ? "zero" : scoreEW > 0 ? "positive" : "negative"}>
        <span>EW</span>
        <strong>{signedScore(scoreEW)}</strong>
      </div>
    </div>
  );
}

function lineupLabel(entry: ScoreSheetEntry) {
  const northSouth = entry.lineup.northSouth.members.map((member) => member.nickname).join(" / ");
  const eastWest = entry.lineup.eastWest.members.map((member) => member.nickname).join(" / ");
  return `NS ${northSouth} · EW ${eastWest}`;
}

function SessionSummary({ table }: { table: LiveTableProjection | null }) {
  if (table === null) {
    return (
      <section className="history-session-summary history-session-empty" aria-labelledby="history-session-title">
        <div>
          <span className="history-kicker">Session</span>
          <h2 id="history-session-title">Tidak ada meja aktif</h2>
          <p>Board yang dimainkan pada sesi ini akan muncul di sini.</p>
        </div>
      </section>
    );
  }

  const scoreNS = table.scoreSheet.reduce((total, entry) => total + entry.result.scoreNS, 0);
  return (
    <section className="history-session-summary" aria-labelledby="history-session-title">
      <div className="history-summary-heading">
        <span className="history-kicker">Session</span>
        <h2 id="history-session-title">Sesi aktif</h2>
      </div>
      <div className="history-summary-metrics" aria-label="Ringkasan sesi">
        <div><span>Boards</span><strong>{table.scoreSheet.length}</strong></div>
        <div data-score-sign={scoreNS === 0 ? "zero" : scoreNS > 0 ? "positive" : "negative"}><span>NS</span><strong>{signedScore(scoreNS)}</strong></div>
        <div data-score-sign={scoreNS === 0 ? "zero" : scoreNS < 0 ? "positive" : "negative"}><span>EW</span><strong>{signedScore(-scoreNS)}</strong></div>
      </div>
      <Link className="history-back-link" href={`/table/${table.tableId}`}><HistoryIcon name="arrow-left" />Kembali ke meja</Link>
    </section>
  );
}

function BoardList({ table, selectedEntry, onSelect }: { table: LiveTableProjection; selectedEntry: ScoreSheetEntry | null; onSelect: (boardId: string) => void }) {
  return (
    <div className="history-board-navigation">
      <div className="history-section-label">
        <span className="history-kicker">History</span>
        <h3>Boards</h3>
      </div>
      {table.scoreSheet.length === 0 ? <p className="history-empty-copy">Belum ada board.</p> : (
        <ol className="history-board-list" aria-label="Board meja aktif">
          {table.scoreSheet.map((entry) => {
            const context = scoreContext(entry, table.viewerSeat);
            const selected = selectedEntry?.boardId === entry.boardId;
            return (
              <li key={entry.boardId}>
                <button className="history-board-row" type="button" aria-pressed={selected} onClick={() => onSelect(entry.boardId)}>
                  <span className="history-board-number">Board {boardNumberLabel(entry.boardNumber)}</span>
                  <strong className="history-board-contract"><ContractValue entry={entry} showDeclarer /></strong>
                  <strong className="history-board-result">{context.resultLabel}</strong>
                  <span className="history-board-score">{context.perspective} {signedScore(context.perspectiveScore)}</span>
                  <span className="history-board-meta">{vulnerabilityLabels[entry.result.vulnerability]}</span>
                  <span className="history-board-meta">{lineupLabel(entry)}</span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}

function BoardDetail({ table, entry, replayAllowed, onOpenReplay }: { table: LiveTableProjection; entry: ScoreSheetEntry | null; replayAllowed: boolean; onOpenReplay: (entry: ScoreSheetEntry, trigger: HTMLButtonElement) => void }) {
  if (entry === null) {
    return (
      <div className="history-board-detail history-board-detail-empty">
        <span className="history-kicker">Selected board</span>
        <h3>Pilih board untuk melihat detail</h3>
        <p>Kontrak, result, score, dan replay akan muncul di sini.</p>
      </div>
    );
  }

  const context = scoreContext(entry, table.viewerSeat);
  return (
    <article className="history-board-detail" aria-labelledby="selected-board-title">
      <header className="history-detail-heading">
        <div>
          <span className="history-kicker">Board {boardNumberLabel(entry.boardNumber)}</span>
          <h3 id="selected-board-title"><ContractValue entry={entry} showDeclarer={false} /></h3>
          {entry.result.passedOut ? null : <p className="history-detail-declarer">Declarer {entry.result.contract?.declarer}</p>}
        </div>
        <span className="history-vulnerability">{vulnerabilityLabels[entry.result.vulnerability]}</span>
      </header>
      <p className="history-detail-players">{lineupLabel(entry)}</p>
      <div className="history-outcome">
        <div className="history-result-block">
          <span className="history-label">Result</span>
          <strong className="history-result">{context.resultLabel}</strong>
          <span className="history-perspective">Perspektif: {context.perspective} {signedScore(context.perspectiveScore)}</span>
        </div>
        <ScorePair scoreNS={entry.result.scoreNS} />
      </div>
      <footer className="history-detail-actions">
        {replayAllowed ? <button className="history-replay-button" type="button" onClick={(event) => onOpenReplay(entry, event.currentTarget)}><HistoryIcon name="play" />Buka replay &amp; analysis</button> : <p>Replay dan analysis tersedia setelah Team Match selesai.</p>}
      </footer>
    </article>
  );
}

function matchScoreLabel(match: MatchView) {
  if (match.status !== "COMPLETE") return `${match.openCompleted}/${match.boardCount} open · ${match.closedCompleted}/${match.boardCount} closed`;
  const comparisonLabel = match.comparisons === undefined ? "Hasil perbandingan tersedia" : `${match.comparisons.length} perbandingan`;
  return `Tim A ${signedScore(match.teamAIMP)} IMP · Tim B ${signedScore(-match.teamAIMP)} IMP · ${comparisonLabel}`;
}

function TeamMatchSection({ matches, loading, error, onRetry }: { matches: MatchView[]; loading: boolean; error: string; onRetry: () => void }) {
  return (
    <section className="history-team-section" aria-labelledby="match-history-title">
      <div className="history-section-heading">
        <div>
          <span className="history-kicker">Matches</span>
          <h2 id="match-history-title">Team Match</h2>
          <p>Match yang dapat kamu akses dari sesi ini.</p>
        </div>
      </div>
      {loading ? <p role="status">Memuat match…</p> : null}
      {error ? <div role="alert"><p className="form-error">{error}</p><button type="button" onClick={onRetry}>Coba lagi</button></div> : null}
      {!loading && !error && matches.length === 0 ? <p className="history-empty-copy">Belum ada Team Match yang dapat diakses.</p> : null}
      <ul className="history-match-list">
        {matches.map((match) => (
          <li key={match.id}>
            <Link href={`/match/${match.id}`}>
              <span className="history-match-participant">Tim {match.team} · {match.room} · {match.seat}</span>
              <span>{matchStatusLabels[match.status]} · {match.boardCount} board</span>
              <strong>{match.status === "WAITING" ? `${match.readyCount}/8 siap` : matchScoreLabel(match)}</strong>
              <span className="history-match-action">Lihat match <span aria-hidden="true">→</span></span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

export function HistoryWorkspace() {
  const session = useTableSession();
  const table = session.projectedTable;
  const presentation = useWorkspacePresentation();
  const [matches, setMatches] = useState<MatchView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  const [replaySelection, setReplaySelection] = useState<ReplaySelection | null>(null);
  const replayTriggerRef = useRef<HTMLButtonElement | null>(null);
  const historyBoardId = presentation?.historyBoardId ?? null;
  const setHistoryBoardId = presentation?.setHistoryBoardId;

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await accountRequest<MatchView[]>("/matches", { signal: controller.signal });
        if (!controller.signal.aborted) {
          setMatches(result);
          setError("");
          setLoading(false);
        }
      } catch (failure) {
        if (!controller.signal.aborted) {
          setError(failure instanceof Error ? failure.message : "Koneksi terputus.");
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [revision]);

  useEffect(() => {
    if (table === null || historyBoardId === null || table.scoreSheet.some((entry) => entry.boardId === historyBoardId)) return;
    setHistoryBoardId?.(null);
  }, [historyBoardId, setHistoryBoardId, table]);

  const selectedEntry = table === null
    ? null
    : historyBoardId === null
      ? table.scoreSheet.at(-1) ?? null
      : table.scoreSheet.find((entry) => entry.boardId === historyBoardId) ?? null;
  const replayAllowed = selectedEntry !== null && !(table?.matchId && !table.matchComplete);
  const replayEntry = replaySelection === null || table === null || replaySelection.tableId !== table.tableId || replaySelection.matchComplete !== (table.matchComplete === true)
    ? null
    : table.scoreSheet.find((entry) => entry.boardId === replaySelection.boardId) ?? null;

  function openReplay(entry: ScoreSheetEntry, trigger: HTMLButtonElement) {
    replayTriggerRef.current = trigger;
    if (table !== null) setReplaySelection({ tableId: table.tableId, boardId: entry.boardId, matchComplete: table.matchComplete === true });
  }

  return (
    <div className="history-workspace">
      <SessionSummary table={table} />
      {table === null ? null : (
        <section className="history-board-section" aria-labelledby="board-history-title">
          <div className="history-section-heading">
            <div>
              <span className="history-kicker">Board history</span>
              <h2 id="board-history-title">Board results</h2>
              <p>Select a board to inspect its result and replay access.</p>
            </div>
          </div>
          <div className="history-board-workspace">
            <BoardList table={table} selectedEntry={selectedEntry} onSelect={(boardId) => setHistoryBoardId?.(boardId)} />
            <BoardDetail table={table} entry={selectedEntry} replayAllowed={replayAllowed} onOpenReplay={openReplay} />
          </div>
          {replayEntry === null ? null : (
            <BoardReplayModal
              key={replayEntry.boardId}
              table={table}
              entry={replayEntry}
              loadBoardReplay={session.loadBoardReplay}
              loadPositionAnalysis={session.loadPositionAnalysis}
              onClose={() => {
                setReplaySelection(null);
                requestAnimationFrame(() => replayTriggerRef.current?.focus());
              }}
            />
          )}
        </section>
      )}
      <TeamMatchSection matches={matches} loading={loading} error={error} onRetry={() => { setLoading(true); setRevision((value) => value + 1); }} />
    </div>
  );
}
