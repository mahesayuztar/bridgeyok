import type { LiveTableProjection } from "../table-state";
import { viewerTrickCounts } from "./gameplay-presentation";

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
  return (
    <span
      className="trick-indicator"
      role="img"
      aria-label={`Trick partnership Anda: ${counts.won} menang, ${counts.lost} kalah`}
      data-won={counts.won}
      data-lost={counts.lost}
      data-partnership={counts.viewerPartnership}
      data-history-available="false"
    >
      <span className="trick-card-count trick-won" aria-hidden="true">
        {counts.won}
      </span>
      <span className="trick-card-count trick-lost" aria-hidden="true">
        {counts.lost}
      </span>
    </span>
  );
}
