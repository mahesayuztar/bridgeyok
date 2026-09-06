import { useRef, useState } from "react";
import type { TableSession } from "../use-table-session";
import { BoardReplayModal } from "./board-replay-modal";
import { boardResultLabel, type LiveTableProjection } from "../table-state";
import { compactContractLabel } from "./gameplay-presentation";

export function ScoreSheet({
  table,
  compact = false,
  loadBoardReplay,
}: {
  table: LiveTableProjection;
  compact?: boolean;
  loadBoardReplay: TableSession["loadBoardReplay"];
}) {
  const selectedTriggerRef = useRef<HTMLButtonElement | null>(null);
  const [selectedEntry, setSelectedEntry] = useState<
    LiveTableProjection["scoreSheet"][number] | null
  >(null);
  const scoreSheetId = `score-sheet-${table.tableId}`;
  const scoreSheetTitleId = `${scoreSheetId}-title`;
  const scoreNS = table.scoreSheet.reduce(
    (total, entry) => total + entry.result.scoreNS,
    0,
  );

  return (
    <>
      <button
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
        <header className="score-sheet-header">
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
                      selectedTriggerRef.current =
                        event.currentTarget.querySelector("button");
                      selectedTriggerRef.current?.focus();
                      setSelectedEntry(entry);
                    }}
                  >
                    <th scope="row">
                      <button
                        className="score-replay-trigger"
                        type="button"
                        aria-label={`Replay board ${entry.boardNumber}`}
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
          loadBoardReplay={loadBoardReplay}
          onClose={() => {
            setSelectedEntry(null);
            document.getElementById(scoreSheetId)?.showPopover();
            selectedTriggerRef.current?.focus();
          }}
        />
      )}
    </>
  );
}
