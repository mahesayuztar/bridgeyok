import { boardResultLabel, type LiveTableProjection } from "../table-state";
import type { TableSession } from "../use-table-session";
import {
  compactContractLabel,
  contractScoreLabel,
} from "./gameplay-presentation";

export function BoardResult({
  table,
  canSendCommand,
  onCommand,
  persistent = false,
}: {
  table: LiveTableProjection;
  persistent?: boolean;
  canSendCommand: TableSession["canSendCommand"];
  onCommand: TableSession["sendCommand"];
}) {
  const result = table.game?.result;
  if (result === undefined) return null;

  return (
    <section
      className="board-result"
      aria-label={`Hasil board ${table.game?.board.number}`}
    >
      <dl>
        <div>
          <dt>Contract</dt>
          <dd>{compactContractLabel(result.contract)}</dd>
        </div>
        <div>
          <dt>Result</dt>
          <dd>{boardResultLabel(result)}</dd>
        </div>
        <div>
          <dt>Score</dt>
          <dd>{contractScoreLabel(result)}</dd>
        </div>
      </dl>
      {persistent || table.viewerRole !== "OWNER" ? null : (
        <button
          type="button"
          className="board-next-button"
          disabled={!canSendCommand("table.next_board")}
          onClick={() => {
            if (canSendCommand("table.next_board")) onCommand("table.next_board");
          }}
        >
          Board berikutnya
        </button>
      )}
      {!persistent && table.viewerRole !== "OWNER" ? (
        <p className="board-next-waiting">Menunggu pemilik meja</p>
      ) : null}
    </section>
  );
}
