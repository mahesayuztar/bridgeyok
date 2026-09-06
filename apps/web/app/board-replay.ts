import type { components } from "@bridgeyok/contracts/openapi";
import { normalizeGame } from "./table-projection.ts";
import type { GameProjection, Seat } from "./table-state.ts";

export type BoardReplay = {
  boardId: string;
  fullDeal: NonNullable<GameProjection["fullDeal"]>;
  game: GameProjection & { claimed: boolean };
};

export function normalizeBoardReplay(
  value: components["schemas"]["BoardReplay"],
): BoardReplay {
  const game = normalizeGame({
    ...value.game,
    ownHand: [],
    completedTrickCount: value.game.completedTricks?.length ?? 0,
    fullDeal: value.fullDeal,
  });
  if (
    game?.phase !== "BOARD_SCORED" ||
    !game.result ||
    !game.fullDeal ||
    typeof value.game.claimed !== "boolean"
  ) {
    throw new Error("Invalid completed board replay");
  }
  const hands = Object.values(game.fullDeal);
  if (
    hands.some((hand) => hand.length !== 13) ||
    new Set(hands.flat().map((card) => `${card.suit}${card.rank}`)).size !== 52
  ) {
    throw new Error("Invalid replay deal");
  }
  return {
    boardId: value.boardId,
    fullDeal: game.fullDeal,
    game: { ...game, claimed: value.game.claimed },
  };
}

export function replayFrame(replay: BoardReplay, step: number) {
  const tricks = replay.game.completedTricks ?? [];
  const position = Math.max(0, Math.min(step, tricks.length + 1));
  const visibleTricks =
    position > tricks.length ? [] : tricks.slice(0, position);
  const playedCards = new Set(
    visibleTricks.flatMap((trick) =>
      trick.plays.map(
        (play) => `${play.seat}:${play.card.suit}${play.card.rank}`,
      ),
    ),
  );
  const seatHands = { N: "north", E: "east", S: "south", W: "west" } as const;
  const hands = Object.fromEntries(
    (Object.keys(seatHands) as Seat[]).map((seat) => [
      seat,
      replay.fullDeal[seatHands[seat]].filter(
        (card) => !playedCards.has(`${seat}:${card.suit}${card.rank}`),
      ),
    ]),
  ) as Record<Seat, GameProjection["ownHand"]>;
  return {
    hands,
    trick:
      position > 0 && position <= tricks.length
        ? tricks[position - 1]
        : undefined,
    showResult: position === tricks.length + 1,
    lastStep: tricks.length + 1,
  };
}
