import type { GameProjection, Trick } from "./table-state.ts";

export type GameplayMotionEvent =
  | { kind: "play"; trick: Trick; movingSeat: NonNullable<Trick["plays"]>[number]["seat"] }
  | { kind: "winner"; trick: Trick }
  | { kind: "collect"; trick: Trick }
  | { kind: "sync"; trick: Trick };

export const GAMEPLAY_MOTION_DURATION = {
  play: 220,
  winnerPause: 3_200,
  collect: 180,
  reducedPlay: 40,
  reducedWinnerPause: 1_200,
  reducedCollect: 40,
} as const;

function cardKey(trick: Trick) {
  return trick.plays
    .map((play) => `${play.seat}:${play.card.suit}${play.card.rank}`)
    .join("|");
}

export function gameplayMotionKey(game: GameProjection) {
  return [
    game.board.number,
    game.phase,
    game.completedTrickCount,
    cardKey(game.currentTrick),
  ].join(":");
}

export function gameplayMotionEvents(
  previous: GameProjection,
  current: GameProjection,
): GameplayMotionEvent[] {
  if (
    previous.board.number !== current.board.number ||
    current.completedTrickCount < previous.completedTrickCount
  ) {
    return [{ kind: "sync", trick: current.currentTrick }];
  }

  const events: GameplayMotionEvent[] = [];
  if (current.completedTrickCount > previous.completedTrickCount) {
    const completedTrick = current.completedTricks.at(-1);
    if (
      current.completedTrickCount !== previous.completedTrickCount + 1 ||
      completedTrick === undefined
    ) {
      return [{ kind: "sync", trick: current.currentTrick }];
    }
    for (
      let _playIndex = previous.currentTrick.plays.length;
      _playIndex < completedTrick.plays.length;
      _playIndex++
    ) {
      const play = completedTrick.plays[_playIndex]!;
      events.push({
        kind: "play",
        trick: {
          ...completedTrick,
          plays: completedTrick.plays.slice(0, _playIndex + 1),
        },
        movingSeat: play.seat,
      });
    }
    events.push({ kind: "winner", trick: completedTrick });
    events.push({ kind: "collect", trick: completedTrick });
    current.currentTrick.plays.forEach((play, _playIndex) => {
      events.push({
        kind: "play",
        trick: {
          ...current.currentTrick,
          plays: current.currentTrick.plays.slice(0, _playIndex + 1),
        },
        movingSeat: play.seat,
      });
    });
    return events;
  }

  const previousCards = cardKey(previous.currentTrick);
  const currentCards = cardKey(current.currentTrick);
  if (
    currentCards.startsWith(previousCards) &&
    current.currentTrick.plays.length >= previous.currentTrick.plays.length
  ) {
    for (
      let _playIndex = previous.currentTrick.plays.length;
      _playIndex < current.currentTrick.plays.length;
      _playIndex++
    ) {
      const play = current.currentTrick.plays[_playIndex]!;
      events.push({
        kind: "play",
        trick: {
          ...current.currentTrick,
          plays: current.currentTrick.plays.slice(0, _playIndex + 1),
        },
        movingSeat: play.seat,
      });
    }
    return events;
  }

  return [{ kind: "sync", trick: current.currentTrick }];
}

export function shouldSkipCompletedTrickPause(
  previous: GameProjection,
  current: GameProjection,
) {
  return (
    previous.completedTrickCount > 0 &&
    current.completedTrickCount === previous.completedTrickCount &&
    previous.currentTrick.plays.length === 0 &&
    current.currentTrick.plays.length > 0
  );
}
