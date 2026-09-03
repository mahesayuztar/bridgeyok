import type { GameProjection, Seat, TableOrientation } from "../table-state";
import { completedDealHands } from "./gameplay-presentation";
import { BridgeHand } from "./playing-card";

export function CompletedDeal({
  game,
  orientation,
}: {
  game: GameProjection;
  orientation: TableOrientation;
}) {
  const hands = completedDealHands(game);
  if (hands === null) return null;

  return (
    <div className="completed-deal" aria-label="Seluruh kartu board">
      {(Object.entries(orientation) as Array<
        [keyof TableOrientation, Seat]
      >).map(([position, seat]) => (
        <BridgeHand
          key={seat}
          className={`dummy-hand dummy-${position} completed-deal-hand completed-deal-${position}`}
          title={`Kartu ${seat}`}
          variant="dummy"
          position={position}
          cards={hands[seat]}
          contractStrain={game.auction.contract?.strain}
        />
      ))}
    </div>
  );
}
