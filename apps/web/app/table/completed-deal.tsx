import type { Card, GameProjection, Seat, TableOrientation } from "../table-state";
import { completedDealHands } from "./gameplay-presentation";
import { BridgeHand } from "./playing-card";

export function CompletedDeal({
  game,
  orientation,
  hands: suppliedHands,
  turn,
  playableCards = [],
  predictions,
  analysisPending = false,
}: {
  game: GameProjection;
  orientation: TableOrientation;
  hands?: Record<Seat, Card[]>;
  turn?: Seat | undefined;
  playableCards?: Card[];
  predictions?: Array<{ card: Card; tricks: number }> | undefined;
  analysisPending?: boolean;
}) {
  const hands = suppliedHands ?? completedDealHands(game);
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
          playableCards={seat === turn ? playableCards : []}
          predictions={seat === turn ? predictions : undefined}
          analysisPending={seat === turn && analysisPending}
          contractStrain={game.auction.contract?.strain}
        />
      ))}
    </div>
  );
}
