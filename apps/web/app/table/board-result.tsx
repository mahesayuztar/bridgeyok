import { boardResultLabel, type LiveTableProjection } from "../table-state";
import {
  compactContractLabel,
  contractScoreLabel,
} from "./gameplay-presentation";

export function BoardResult({ table }: { table: LiveTableProjection }) {
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
    </section>
  );
}
