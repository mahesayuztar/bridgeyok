import {
  visualPositionForSeat,
  type Seat,
  type TableOrientation,
  type Trick,
  type VisualPosition,
} from "../table-state";
import { PlayingCard } from "./playing-card";

export function CurrentTrick({
  trick,
  orientation,
  stage = "idle",
  movingSeat,
  dummyPosition,
}: {
  trick: Trick;
  orientation: TableOrientation;
  stage?: "idle" | "moving" | "winner" | "collecting";
  movingSeat?: Seat;
  dummyPosition?: VisualPosition;
}) {
  return (
    <div
      className="current-trick"
      aria-label="Trick saat ini"
      data-motion-stage={stage}
      data-winner={trick.winner}
      data-dummy-position={dummyPosition}
    >
      <span className="trick-center">
        {trick.plays.length === 0 ? "Lead" : ""}
      </span>
      {trick.plays.map((play) => (
        <div
          className={`trick-slot trick-${visualPositionForSeat(orientation, play.seat)}`}
          key={play.seat}
          data-moving={stage === "moving" && movingSeat === play.seat}
          data-winner={stage === "winner" && trick.winner === play.seat}
        >
          <span>{play.seat}</span>
          <PlayingCard card={play.card} variant="trick" />
        </div>
      ))}
    </div>
  );
}
