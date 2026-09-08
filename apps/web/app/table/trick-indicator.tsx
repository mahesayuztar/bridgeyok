import { useDialogDrag } from "./use-dialog-drag";
import { useState } from "react";
import {
  oppositeSeat,
  tableOrientation,
  visualPositionForSeat,
  type LiveTableProjection,
} from "../table-state";
import { cardKey, viewerTrickCounts } from "./gameplay-presentation";
import { PlayingCard } from "./playing-card";

export function TrickIndicator({ table }: { table: LiveTableProjection }) {
  const dialogDrag = useDialogDrag();
  const [selectedTrickNumber, setSelectedTrickNumber] = useState<number | null>(
    null,
  );
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
  const trickNumber = Math.min(
    game.completedTrickCount,
    Math.max(
      firstVisibleTrickNumber,
      selectedTrickNumber ?? game.completedTrickCount,
    ),
  );
  const trick = game.completedTricks[trickNumber - firstVisibleTrickNumber]!;
  const orientation = tableOrientation(table.viewerSeat);

  return (
    <>
      <button
        className="trick-indicator-button"
        type="button"
        popoverTarget={historyId}
        aria-label={`Trick partnership Anda: ${counts.won} menang, ${counts.lost} kalah. Buka riwayat trick`}
        data-history-available="true"
        onClick={() => setSelectedTrickNumber(null)}
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
        <header {...dialogDrag} className="trick-history-header">
          <h2 id={historyTitleId}>Trick {trickNumber}</h2>
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
        <ol className="trick-history-list">
          <li className="trick-history-item" key={trickNumber}>
            <div className="trick-history-cards">
              {trick.plays.map((play) => (
                <div
                  className={`trick-history-play history-${visualPositionForSeat(orientation, play.seat)}`}
                  key={`${play.seat}-${cardKey(play.card)}`}
                >
                  <span>{play.seat}</span>
                  <PlayingCard card={play.card} variant="trick" />
                </div>
              ))}
            </div>
          </li>
        </ol>
        <nav className="trick-history-navigation" aria-label="Navigasi trick">
          <button
            type="button"
            aria-label="Trick sebelumnya"
            disabled={trickNumber === firstVisibleTrickNumber}
            onClick={() => setSelectedTrickNumber(trickNumber - 1)}
          >
            ‹
          </button>
          <button
            type="button"
            aria-label="Trick berikutnya"
            disabled={trickNumber === game.completedTrickCount}
            onClick={() => setSelectedTrickNumber(trickNumber + 1)}
          >
            ›
          </button>
        </nav>
      </section>
    </>
  );
}
