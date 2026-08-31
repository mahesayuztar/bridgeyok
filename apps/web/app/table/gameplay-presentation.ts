import type { Call, Card, LiveTableProjection, Suit } from "../table-state";

export const suitLabels: Record<Suit, string> = {
  S: "♠",
  H: "♥",
  D: "♦",
  C: "♣",
};

const cardRankValue: Record<Card["rank"], number> = {
  A: 14,
  K: 13,
  Q: 12,
  J: 11,
  T: 10,
  "9": 9,
  "8": 8,
  "7": 7,
  "6": 6,
  "5": 5,
  "4": 4,
  "3": 3,
  "2": 2,
};

export function sortCardsDescending(cards: Card[]) {
  return [...cards].sort(
    (firstCard, secondCard) =>
      cardRankValue[secondCard.rank] - cardRankValue[firstCard.rank],
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
  includeDeclarer = true,
) {
  if (contract === undefined) return "Belum ada kontrak";
  const doubling =
    contract.doubling === "DOUBLED"
      ? " X"
      : contract.doubling === "REDOUBLED"
        ? " XX"
        : "";
  const strain = contract.strain === "NT" ? "NT" : suitLabels[contract.strain];
  return `${contract.level}${strain}${doubling}${includeDeclarer ? ` oleh ${contract.declarer}` : ""}`;
}

export function cardKey(card: Card) {
  return `${card.suit}${card.rank}`;
}
