import { useDialogDrag } from "./use-dialog-drag";
import { useRef, useState } from "react";
import type { TableSession } from "../use-table-session";
import { BoardReplayModal } from "./board-replay-modal";
import { boardResultLabel, type LiveTableProjection } from "../table-state";
import { compactContractLabel } from "./gameplay-presentation";

export function ScoreSheet({
  table,
  compact = false,
  loadBoardReplay,
  loadPositionAnalysis,
}: {
  table: LiveTableProjection;
  compact?: boolean;
  loadBoardReplay: TableSession["loadBoardReplay"];
  loadPositionAnalysis: TableSession["loadPositionAnalysis"];
}) {
  const dialogDrag = useDialogDrag();
  const scoreSheetTriggerRef = useRef<HTMLButtonElement | null>(null);
  const selectedTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [selectedBoardId, setSelectedBoardId] = useState<string | null>(null);
  const scoreSheetId = `score-sheet-${table.tableId}`;
  const scoreSheetTitleId = `${scoreSheetId}-title`;
  const scoreNS = table.scoreSheet.reduce(
    (total, entry) => total + entry.result.scoreNS,
    0,
  );
  const selectedEntry = selectedBoardId === null
    ? null
    : table.scoreSheet.find((entry) => entry.boardId === selectedBoardId) ?? null;

  function selectEntry(
    entry: LiveTableProjection["scoreSheet"][number],
    trigger: HTMLButtonElement,
  ) {
    if (table.matchId && !table.matchComplete) return;
    selectedTriggerRef.current = trigger;
    document.getElementById(scoreSheetId)?.hidePopover();
    setSelectedBoardId(entry.boardId);
  }

  function restoreScoreSheetFocus() {
    const trigger = selectedTriggerRef.current;
    window.requestAnimationFrame(() => {
      document.getElementById(scoreSheetId)?.showPopover();
      const focusTarget = trigger?.isConnected && !trigger.disabled && trigger.getClientRects().length > 0
        ? trigger
        : scoreSheetTriggerRef.current;
      if (focusTarget?.isConnected && !focusTarget.disabled && focusTarget.getClientRects().length > 0) {
        focusTarget.focus();
      }
    });
  }

  return (
    <>
      <button
        ref={scoreSheetTriggerRef}
        className={
          compact ? "score-sheet-trigger score-summary" : "score-sheet-trigger"
        }
        type="button"
        popoverTarget={scoreSheetId}
        aria-label="Buka skor meja"
        aria-description={
          compact ? `Poin NS ${scoreNS}, EW ${-scoreNS || 0}` : undefined
        }
        aria-haspopup="dialog"
      >
        {compact ? (
          <>
            <span>Poin</span>
            <span>
              NS <strong>{scoreNS}</strong>
            </span>
            <span>
              EW <strong>{-scoreNS || 0}</strong>
            </span>
          </>
        ) : (
          "History"
        )}
      </button>
      <div
        className="score-sheet-popover"
        id={scoreSheetId}
        popover="auto"
        role="dialog"
        aria-labelledby={scoreSheetTitleId}
      >
        <header {...dialogDrag} className="score-sheet-header">
          <h2 id={scoreSheetTitleId}>History</h2>
          <button
            className="score-sheet-close"
            type="button"
            popoverTarget={scoreSheetId}
            popoverTargetAction="hide"
            aria-label="Tutup skor meja"
          >
            ×
          </button>
        </header>

        {table.scoreSheet.length === 0 ? (
          <p className="score-sheet-empty">Belum ada hasil board.</p>
        ) : (
          <div
            className="score-sheet-table-wrap"
            role="region"
            aria-label="Hasil board meja ini"
            tabIndex={0}
          >
            <table className="score-sheet-table">
              <caption>Hasil board meja ini</caption>
              <thead>
                <tr>
                  <th scope="col">Board</th>
                  <th scope="col">Kontrak</th>
                  <th scope="col">NS</th>
                  <th scope="col">EW</th>
                </tr>
              </thead>
              <tbody>
                {table.scoreSheet.map((entry) => (
                  <tr
                    key={entry.boardId}
                    onClick={(event) => {
                      if (event.target instanceof Element && event.target.closest("button")) return;
                      const trigger = event.currentTarget.querySelector("button");
                      if (trigger) selectEntry(entry, trigger);
                    }}
                  >
                    <th scope="row">
                      <button
                        disabled={Boolean(table.matchId && !table.matchComplete)}
                        title={table.matchId && !table.matchComplete ? "Replay terbuka setelah kedua room selesai" : undefined}
                        className="score-replay-trigger"
                        type="button"
                        aria-label={`Replay board ${entry.boardNumber}`}
                        onClick={(event) => {
                          event.stopPropagation();
                          selectEntry(entry, event.currentTarget);
                        }}
                      >
                        {entry.boardNumber}
                      </button>
                    </th>
                    <td data-strain={entry.result.contract?.strain}>
                      {compactContractLabel(entry.result.contract)}
                      {entry.result.contract === undefined
                        ? ""
                        : boardResultLabel(entry.result)}
                    </td>
                    <td>
                      {entry.result.scoreNS >= 0 ? entry.result.scoreNS : ""}
                    </td>
                    <td>
                      {entry.result.scoreNS < 0 ? -entry.result.scoreNS : ""}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      {selectedEntry === null ? null : (
        <BoardReplayModal
          key={selectedEntry.boardId}
          table={table}
          entry={selectedEntry}
          loadBoardReplay={loadBoardReplay} loadPositionAnalysis={loadPositionAnalysis}
          onClose={() => {
            setSelectedBoardId(null);
            restoreScoreSheetFocus();
          }}
        />
      )}
    </>
  );
}
