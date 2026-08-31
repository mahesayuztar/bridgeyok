import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import { contractLabel } from "./gameplay-presentation";

export function BoardResult({
  table,
  commandDisabled,
  onCommand,
}: {
  table: LiveTableProjection;
  commandDisabled: boolean;
  onCommand: TableSession["sendCommand"];
}) {
  const result = table.game?.result;
  if (result === undefined) return null;
  return (
    <section className="board-result" aria-labelledby="result-title">
      <p>Board {table.game?.board.number} selesai</p>
      <h2 id="result-title">
        {result.passedOut ? "Passed out" : contractLabel(result.contract, false)}
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
            disabled={commandDisabled}
            onClick={() => onCommand("table.next_board")}
          >
            Board berikutnya
          </button>
          <button
            type="button"
            disabled={commandDisabled}
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
