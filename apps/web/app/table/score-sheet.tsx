import { boardResultLabel, type LiveTableProjection } from "../table-state";
import { compactContractLabel } from "./gameplay-presentation";

function scoreLabel(score: number) {
  return score > 0 ? `+${score}` : String(score);
}

function pairLabel(pair: LiveTableProjection["pairScoreTotals"][number]["pair"]) {
  return pair.members.map((member) => member.nickname).join(" & ");
}

export function ScoreSheet({ table }: { table: LiveTableProjection }) {
  const scoreSheetId = `score-sheet-${table.tableId}`;
  const scoreSheetTitleId = `${scoreSheetId}-title`;
  const viewerTotals = table.pairScoreTotals.filter((total) =>
    total.pair.members.some((member) => member.id === table.viewerParticipantId),
  );
  const viewerTotal = viewerTotals.length === 1 ? viewerTotals[0] : undefined;

  return (
    <>
      <button
        className="score-sheet-trigger"
        type="button"
        popoverTarget={scoreSheetId}
        aria-label="Buka skor meja"
        aria-haspopup="dialog"
      >
        <span>Skor</span>
        {viewerTotal === undefined ? null : (
          <strong>{scoreLabel(viewerTotal.score)}</strong>
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
          <div>
            <p className="eyebrow">Duplicate points · bukan IMP</p>
            <h2 id={scoreSheetTitleId}>Skor meja</h2>
          </div>
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

        {table.pairScoreTotals.length === 0 ? null : (
          <section className="score-sheet-totals" aria-labelledby={`${scoreSheetId}-totals`}>
            <h3 id={`${scoreSheetId}-totals`}>Total pasangan</h3>
            <dl>
              {table.pairScoreTotals.map((total) => (
                <div key={total.pair.id} data-viewer-pair={total.pair.members.some((member) => member.id === table.viewerParticipantId)}>
                  <dt>{pairLabel(total.pair)}</dt>
                  <dd>{scoreLabel(total.score)}</dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        {table.scoreSheet.length === 0 ? (
          <p className="score-sheet-empty">Belum ada hasil board.</p>
        ) : (
          <div className="score-sheet-table-wrap" role="region" aria-label="Hasil board meja ini" tabIndex={0}>
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
                  <tr key={entry.boardId}>
                    <th scope="row">{entry.boardNumber}</th>
                    <td>
                      <strong>{compactContractLabel(entry.result.contract)}</strong>
                      <span>{boardResultLabel(entry.result)}</span>
                    </td>
                    <td>
                      <strong>{scoreLabel(entry.result.scoreNS)}</strong>
                      <span>{pairLabel(entry.lineup.northSouth)}</span>
                    </td>
                    <td>
                      <strong>{scoreLabel(-entry.result.scoreNS)}</strong>
                      <span>{pairLabel(entry.lineup.eastWest)}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}
