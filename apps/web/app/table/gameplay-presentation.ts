import type { Call, Card, LiveTableProjection, Suit } from "../table-state";

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
    (firstCard, secondCard) =>
      suitOrder.indexOf(firstCard.suit) - suitOrder.indexOf(secondCard.suit),
  );
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
