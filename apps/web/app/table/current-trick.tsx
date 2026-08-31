import {
  visualPositionForSeat,
  type LiveTableProjection,
  type TableOrientation,
} from "../table-state";
import { PlayingCard } from "./playing-card";

export function CurrentTrick({
  game,
  orientation,
}: {
  game: NonNullable<LiveTableProjection["game"]>;
  orientation: TableOrientation;
}) {
  return (
    <div className="current-trick" aria-label="Trick saat ini">
      <span className="trick-center">
        {game.currentTrick.plays.length === 0 ? "Lead" : ""}
      </span>
      {game.currentTrick.plays.map((play) => (
        <div
          className={`trick-slot trick-${visualPositionForSeat(orientation, play.seat)}`}
          key={play.seat}
        >
          <span>{play.seat}</span>
          <PlayingCard card={play.card} variant="trick" />
        </div>
      ))}
    </div>
  );
}
