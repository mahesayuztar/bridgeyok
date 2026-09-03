import type {
  Call,
  Card,
  GameProjection,
  LiveTableProjection,
  Seat,
  Suit,
} from "../table-state";

export const suitLabels: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const CONTRACT_SUIT_ORDER: Record<
  NonNullable<Call["strain"]>,
  readonly Suit[]
> = {
  NT: ["S", "H", "D", "C"],
  S: ["S", "H", "D", "C"],
  H: ["H", "S", "C", "D"],
  D: ["D", "S", "H", "C"],
  C: ["C", "H", "S", "D"],
};

const RANK_ORDER: Record<Card["rank"], number> = {
  "2": 2,
  "3": 3,
  "4": 4,
  "5": 5,
  "6": 6,
  "7": 7,
  "8": 8,
  "9": 9,
  T: 10,
  J: 11,
  Q: 12,
  K: 13,
  A: 14,
};

export function suitOrderForContract(
  strain?: NonNullable<Call["strain"]>,
) {
  return CONTRACT_SUIT_ORDER[strain ?? "NT"];
}

export function organizeCardsForContract(
  cards: Card[],
  strain?: NonNullable<Call["strain"]>,
) {
  const suitOrder = suitOrderForContract(strain);
  return [...cards].sort(
    (firstCard, secondCard) => {
      const suitDifference =
        suitOrder.indexOf(firstCard.suit) - suitOrder.indexOf(secondCard.suit);
      return suitDifference === 0
        ? RANK_ORDER[secondCard.rank] - RANK_ORDER[firstCard.rank]
        : suitDifference;
    },
  );
}

export function groupCardsForContract(
  cards: Card[],
  strain?: NonNullable<Call["strain"]>,
) {
  const organizedCards = organizeCardsForContract(cards, strain);
  return suitOrderForContract(strain)
    .map((suit, _suitIndex) => {
      const suitCards = organizedCards
        .filter((card) => card.suit === suit)
        .sort(
          (firstCard, secondCard) =>
            RANK_ORDER[secondCard.rank] - RANK_ORDER[firstCard.rank],
        );
      return {
        key: suit,
        suit,
        suitIndex: _suitIndex,
        cards: suitCards,
      };
    })
    .filter((cardGroup) => cardGroup.cards.length > 0);
}

export function callLabel(call: Call) {
  if (call.kind === "PASS") return "Pass";
  if (call.kind === "DOUBLE") return "X";
  if (call.kind === "REDOUBLE") return "XX";
  const strain = call.strain === "NT" ? "NT" : suitLabels[call.strain ?? "C"];
  return `${call.level}${strain}`;
}

export function callKey(call: Call) {
  return `${call.kind}:${call.level ?? ""}:${call.strain ?? ""}`;
}

export function contractLabel(
  contract: NonNullable<LiveTableProjection["game"]>["auction"]["contract"],
) {
  if (contract === undefined) return "Belum ada kontrak";
  const doubling =
    contract.doubling === "DOUBLED"
      ? " X"
      : contract.doubling === "REDOUBLED"
        ? " XX"
        : "";
  const strain = contract.strain === "NT" ? "NT" : suitLabels[contract.strain];
  return `${contract.level}${strain}${doubling}`;
}

export function contractSummaryLabel(
  contract: NonNullable<LiveTableProjection["game"]>["auction"]["contract"],
) {
  if (contract === undefined) return "Belum ada kontrak";
  return `${contractLabel(contract)} ${contract.declarer}`;
}

export function compactContractLabel(
  contract: NonNullable<LiveTableProjection["game"]>["auction"]["contract"],
) {
  if (contract === undefined) return "Passed out";
  const doubling =
    contract.doubling === "DOUBLED"
      ? "x"
      : contract.doubling === "REDOUBLED"
        ? "xx"
        : "";
  const strain = contract.strain === "NT" ? "NT" : suitLabels[contract.strain];
  return `${contract.level}${strain}${doubling}${contract.declarer}`;
}

export function contractScoreLabel(
  result: NonNullable<NonNullable<LiveTableProjection["game"]>["result"]>,
) {
  if (result.contract === undefined) return "0";
  const declarerIsNS =
    result.contract.declarer === "N" || result.contract.declarer === "S";
  const partnership = declarerIsNS ? "NS" : "EW";
  const score = declarerIsNS ? result.scoreNS : -result.scoreNS;
  return `${score > 0 ? "+" : ""}${score} ${partnership}`;
}

export function completedDealHands(game: GameProjection) {
  if (game.phase !== "BOARD_SCORED" || game.fullDeal === undefined) return null;

  const hands: Record<Seat, Card[]> = {
    N: [...game.fullDeal.north],
    E: [...game.fullDeal.east],
    S: [...game.fullDeal.south],
    W: [...game.fullDeal.west],
  };
  const playedCards = [
    ...game.completedTricks.flatMap((trick) => trick.plays),
    ...game.currentTrick.plays,
  ];

  playedCards.forEach((play) => {
    if (!hands[play.seat].some((card) => cardKey(card) === cardKey(play.card))) {
      hands[play.seat].push(play.card);
    }
  });

  return hands;
}

export function viewerTrickCounts(table: LiveTableProjection) {
  const game = table.game;
  if (game === undefined) return null;
  const viewerPartnership =
    table.viewerSeat === "N" || table.viewerSeat === "S"
      ? "NS"
      : table.viewerSeat === "E" || table.viewerSeat === "W"
        ? "EW"
        : null;
  if (viewerPartnership === null) {
    return {
      viewerPartnership,
      won: game.tricksNS,
      lost: game.tricksEW,
    };
  }
  const viewerIsNS = viewerPartnership === "NS";
  return {
    viewerPartnership,
    won: viewerIsNS ? game.tricksNS : game.tricksEW,
    lost: viewerIsNS ? game.tricksEW : game.tricksNS,
  };
}

export function cardKey(card: Card) {
  return `${card.suit}${card.rank}`;
}
