import { oppositeSeat, type LiveTableProjection } from "../table-state";
import {
  cardKey,
  viewerTrickCounts,
} from "./gameplay-presentation";
import { PlayingCard } from "./playing-card";

export function TrickIndicator({ table }: { table: LiveTableProjection }) {
  const counts = viewerTrickCounts(table);
  if (counts === null) return <span>—</span>;
  if (counts.viewerPartnership === null) {
    return (
      <span aria-label={`Trick NS ${counts.won}, EW ${counts.lost}`}>
        {counts.won}–{counts.lost}
      </span>
    );
  }
  const game = table.game;
  const hasHistory =
    game !== undefined &&
    game.completedTrickCount > 0 &&
    game.completedTricks.length > 0;
  const indicator = (
    <span
      className="trick-indicator"
      role="img"
      aria-label={`Trick partnership Anda: ${counts.won} menang, ${counts.lost} kalah`}
      data-won={counts.won}
      data-lost={counts.lost}
      data-partnership={counts.viewerPartnership}
      data-history-available={hasHistory}
    >
      <span className="trick-card-count trick-won" aria-hidden="true">
        {counts.won}
      </span>
      <span className="trick-card-count trick-lost" aria-hidden="true">
        {counts.lost}
      </span>
    </span>
  );
  if (!hasHistory) {
    return indicator;
  }

  const historyId = `trick-history-${table.tableId}`;
  const historyTitleId = `${historyId}-title`;
  const viewerIsDummy =
    table.viewerSeat !== undefined &&
    game.auction.contract !== undefined &&
    table.viewerSeat === oppositeSeat(game.auction.contract.declarer);
  const firstVisibleTrickNumber =
    game.completedTrickCount - game.completedTricks.length + 1;

  return (
    <>
      <button
        className="trick-indicator-button"
        type="button"
        popoverTarget={historyId}
        aria-label={`Trick partnership Anda: ${counts.won} menang, ${counts.lost} kalah. Buka riwayat trick`}
        data-history-available="true"
      >
        {indicator}
      </button>
      <section
        className="trick-history-popover"
        id={historyId}
        popover="auto"
        role="dialog"
        aria-labelledby={historyTitleId}
        data-history-policy={viewerIsDummy ? "full" : "latest"}
      >
        <header className="trick-history-header">
          <div>
            <p className="eyebrow">Permainan kartu</p>
            <h2 id={historyTitleId}>Riwayat trick</h2>
          </div>
          <button
            className="trick-history-close"
            type="button"
            popoverTarget={historyId}
            popoverTargetAction="hide"
            aria-label="Tutup riwayat trick"
          >
            ×
          </button>
        </header>
        <p className="trick-history-policy">
          {viewerIsDummy
            ? `Semua ${game.completedTrickCount} trick selesai tersedia untuk Dummy.`
            : "Hanya trick terakhir yang tersedia untuk posisi Anda."}
        </p>
        <ol className="trick-history-list">
          {game.completedTricks.map((trick, _trickIndex) => {
            const trickNumber = firstVisibleTrickNumber + _trickIndex;
            return (
              <li className="trick-history-item" key={trickNumber}>
                <div className="trick-history-summary">
                  <strong>Trick {trickNumber}</strong>
                  <span>Pemenang {trick.winner ?? "—"}</span>
                </div>
                <div className="trick-history-cards">
                  {trick.plays.map((play) => (
                    <div
                      className="trick-history-play"
                      key={`${play.seat}-${cardKey(play.card)}`}
                    >
                      <span>{play.seat}</span>
                      <PlayingCard card={play.card} variant="trick" />
                    </div>
                  ))}
                </div>
              </li>
            );
          })}
        </ol>
      </section>
    </>
  );
}
