import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import {
  contractSummaryLabel,
  participantNameForSeat,
} from "./gameplay-presentation";

export function BoardResult({
  table,
  canSendCommand,
  onCommand,
}: {
  table: LiveTableProjection;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
}) {
  const result = table.game?.result;
  if (result === undefined) return null;
  const declarerName =
    result.contract === undefined
      ? undefined
      : participantNameForSeat(table, result.contract.declarer);
  return (
    <section className="board-result" aria-labelledby="result-title">
      <p>Board {table.game?.board.number} selesai</p>
      <h2 id="result-title">
        {result.passedOut
          ? "Passed out"
          : contractSummaryLabel(result.contract, declarerName)}
      </h2>
      <strong className="result-outcome">{boardResultLabel(result)}</strong>
      <dl>
        <div>
          <dt>Declarer</dt>
          <dd>{result.contract?.declarer ?? "—"}</dd>
        </div>
        <div>
          <dt>Trick</dt>
          <dd>{result.tricksDeclarer}</dd>
        </div>
        <div>
          <dt>NS</dt>
          <dd>
            {result.scoreNS > 0 ? "+" : ""}
            {result.scoreNS}
          </dd>
        </div>
        <div>
          <dt>EW</dt>
          <dd>
            {result.scoreNS < 0 ? "+" : ""}
            {-result.scoreNS}
          </dd>
        </div>
      </dl>
      {table.viewerRole === "OWNER" && table.state === "BETWEEN_BOARDS" ? (
        <div className="result-actions">
          <button
            className="primary-button"
            type="button"
            disabled={!canSendCommand("table.next_board")}
            onClick={() => onCommand("table.next_board")}
          >
            Board berikutnya
          </button>
          <button
            type="button"
            disabled={!canSendCommand("table.finish")}
            onClick={() => onCommand("table.finish")}
          >
            Akhiri meja
          </button>
        </div>
      ) : (
        <p className="result-waiting">
          Menunggu pemilik menentukan board berikutnya.
        </p>
      )}
    </section>
  );
}
