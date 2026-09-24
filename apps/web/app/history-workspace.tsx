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
    scoreLabel: `NS ${signedScore(result.scoreNS)} · EW ${signedScore(-result.scoreNS)}`,
  };
}

function matchScoreLabel(match: MatchView) {
  if (match.status !== "COMPLETE") {
    return `${match.openCompleted}/${match.boardCount} open · ${match.closedCompleted}/${match.boardCount} closed`;
  }
  const comparisonLabel = match.comparisons === undefined ? "Hasil perbandingan tersedia" : `${match.comparisons.length} perbandingan`;
  return `Tim A ${signedScore(match.teamAIMP)} IMP · Tim B ${signedScore(-match.teamAIMP)} IMP · ${comparisonLabel}`;
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
  const selectedContext = selectedEntry === null || table === null ? null : scoreContext(selectedEntry, table.viewerSeat);
  const replayEntry = replaySelection === null || table === null || replaySelection.tableId !== table.tableId || replaySelection.matchComplete !== (table.matchComplete === true)
    ? null
    : table.scoreSheet.find((entry) => entry.boardId === replaySelection.boardId) ?? null;

  return (
    <div className="history-workspace">
      {table === null ? null : (
        <section className="history-section" aria-labelledby="active-table-history-title">
          <div className="history-section-heading">
            <div>
              <h2 id="active-table-history-title">Board meja aktif</h2>
              <p>Score sheet dari sesi yang sedang berjalan. Pilih board untuk melihat konteks hasilnya.</p>
            </div>
            <Link href={`/table/${table.tableId}`}>Kembali ke meja</Link>
          </div>
          {table.scoreSheet.length === 0 ? <p>Belum ada hasil board di meja ini.</p> : (
            <ol className="history-board-list" aria-label="Board meja aktif">
              {table.scoreSheet.map((entry) => {
                const context = scoreContext(entry, table.viewerSeat);
                const selected = selectedEntry?.boardId === entry.boardId;
                return (
                  <li key={entry.boardId}>
                    <button
                      className="history-board-row"
                      type="button"
                      aria-pressed={selected}
                      onClick={() => setHistoryBoardId?.(entry.boardId)}
                    >
                      <span className="history-board-number">Board {entry.boardNumber}</span>
                      <span className="history-board-chain">
                        {entry.result.passedOut ? <strong>Passed out</strong> : entry.result.contract === undefined ? <strong>Kontrak tidak tersedia</strong> : <strong>{contractLabel(entry.result.contract)} · {entry.result.contract.declarer}</strong>}
                        <span>{vulnerabilityLabels[entry.result.vulnerability]}</span>
                        <span>{context.resultLabel}</span>
                        <span>{context.scoreLabel}</span>
                      </span>
                      <span className="history-board-perspective">{context.perspective} {signedScore(context.perspectiveScore)}</span>
                    </button>
                  </li>
                );
              })}
            </ol>
          )}
          {selectedEntry === null || selectedContext === null ? null : (
            <article className="history-board-detail" aria-labelledby="selected-board-title">
              <div className="history-detail-heading">
                <div>
                  <h3 id="selected-board-title">Board {selectedEntry.boardNumber}</h3>
                  <p>{selectedEntry.lineup.northSouth.members.map((member) => member.nickname).join(" / ")} · NS &nbsp; {selectedEntry.lineup.eastWest.members.map((member) => member.nickname).join(" / ")} · EW</p>
                </div>
                {replayAllowed ? <button type="button" onClick={(event) => { replayTriggerRef.current = event.currentTarget; setReplaySelection({ tableId: table.tableId, boardId: selectedEntry.boardId, matchComplete: table.matchComplete === true }); }}>Buka replay dan analysis</button> : null}
              </div>
              <dl className="history-score-context">
                <div><dt>Kontrak / declarer</dt><dd>{selectedEntry.result.passedOut ? "Passed out" : selectedEntry.result.contract === undefined ? "Kontrak tidak tersedia" : `${contractLabel(selectedEntry.result.contract)} · ${selectedEntry.result.contract.declarer}`}</dd></div>
                <div><dt>Vulnerability</dt><dd>{vulnerabilityLabels[selectedEntry.result.vulnerability]}</dd></div>
                <div><dt>Result</dt><dd>{selectedContext.resultLabel}</dd></div>
                <div><dt>Signed score</dt><dd data-score-sign={selectedEntry.result.scoreNS === 0 ? "zero" : selectedEntry.result.scoreNS > 0 ? "positive" : "negative"}><strong>{selectedContext.scoreLabel}</strong><span>Perspektifmu: {selectedContext.perspective} {signedScore(selectedContext.perspectiveScore)}</span></dd></div>
              </dl>
              {!replayAllowed ? <p className="history-replay-note">Replay dan analysis tersedia setelah Team Match selesai.</p> : null}
            </article>
          )}
          {replayEntry === null || table === null ? null : (
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
      <section className="history-section" aria-labelledby="match-history-title">
        <div className="history-section-heading">
          <div>
            <h2 id="match-history-title">Team Match</h2>
            <p>Match yang dapat kamu akses. Hasil memakai perbandingan dan IMP yang diberikan sistem.</p>
          </div>
        </div>
        {loading ? <p role="status">Memuat match…</p> : null}
        {error ? <div role="alert"><p className="form-error">{error}</p><button type="button" onClick={() => { setLoading(true); setRevision((value) => value + 1); }}>Coba lagi</button></div> : null}
        {!loading && !error && matches.length === 0 ? <p>Belum ada Team Match yang dapat diakses.</p> : null}
        <ul className="history-match-list">
          {matches.map((match) => (
            <li key={match.id}>
              <Link href={`/match/${match.id}`}>
                <strong>Tim {match.team} · {match.room} · {match.seat}</strong>
                <span>{matchStatusLabels[match.status]} · {match.boardCount} board</span>
                <span>{match.status === "WAITING" ? `${match.readyCount}/8 siap` : matchScoreLabel(match)}</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
