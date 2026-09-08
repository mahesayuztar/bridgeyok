import type { components } from "@bridgeyok/contracts/openapi";
import { normalizeGame } from "./table-projection.ts";
import type { GameProjection, Seat, Trick } from "./table-state.ts";

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
  const allPlays = [...tricks.flatMap((trick) => trick.plays), ...(replay.game.currentTrick?.plays ?? [])];
  const position = Math.max(0, Math.min(step, allPlays.length + 1));
  const showResult = position === allPlays.length + 1;
  const playedCards = new Set(
    (showResult ? [] : allPlays.slice(0, position)).map(
      (play) => `${play.seat}:${play.card.suit}${play.card.rank}`,
    ),
  );
  const completedCount = Math.floor(Math.min(position, allPlays.length) / 4);
  const partialPlays = allPlays.slice(completedCount * 4, Math.min(position, allPlays.length));
  const completed = tricks[completedCount - 1];
  const turn = showResult || position === 52 ? undefined
    : partialPlays.length > 0
      ? (["N", "E", "S", "W"] as Seat[])[(["N", "E", "S", "W"].indexOf(partialPlays.at(-1)!.seat) + 1) % 4]
      : completed?.winner ?? allPlays[0]?.seat ?? (replay.game.auction?.contract ? (["N", "E", "S", "W"] as Seat[])[(["N", "E", "S", "W"].indexOf(replay.game.auction.contract.declarer) + 1) % 4] : undefined);
  const leader = partialPlays[0]?.seat ?? turn;
  const currentTrick: Trick = { ...(leader === undefined ? {} : { leader }), plays: partialPlays };
  const seatHands = { N: "north", E: "east", S: "south", W: "west" } as const;
  const hands = Object.fromEntries(
    (Object.keys(seatHands) as Seat[]).map((seat) => [
      seat,
      replay.fullDeal[seatHands[seat]].filter(
        (card) => !playedCards.has(`${seat}:${card.suit}${card.rank}`),
      ),
    ]),
  ) as Record<Seat, GameProjection["ownHand"]>;
  const completedTricks = tricks.slice(0, completedCount);
  const tricksNS = completedTricks.filter((trick) => trick.winner === "N" || trick.winner === "S").length;
  const { result: finalResult, turn: finalTurn, ...baseGame } = replay.game;
  void finalTurn;
  const game: GameProjection = showResult || position === 52 ? replay.game : {
    ...baseGame,
    phase: replay.game.auction?.passedOut ? "BOARD_SCORED" : position === 0 ? "OPENING_LEAD" : "PLAY",
    ...(turn === undefined ? {} : { turn }),
    ...(replay.game.auction?.passedOut && finalResult ? { result: finalResult } : {}),
    currentTrick,
    dummyRevealed: position > 0,
    completedTricks,
    completedTrickCount: completedCount,
    tricksNS,
    tricksEW: completedCount - tricksNS,
  };
  return {
    game,
    hands,
    turn,
    currentTrick,
    dummyRevealed: !showResult && position > 0,
    trick: showResult ? undefined : partialPlays.length > 0 ? currentTrick : completed,
    showResult,
    lastStep: allPlays.length + 1,
  };
}
