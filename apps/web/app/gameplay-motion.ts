import type { GameProjection, Trick } from "./table-state.ts";

export type GameplayMotionEvent =
  | { kind: "play"; trick: Trick; movingSeat: NonNullable<Trick["plays"]>[number]["seat"] }
  | { kind: "winner"; trick: Trick }
  | { kind: "collect"; trick: Trick }
  | { kind: "sync"; trick: Trick };

function cardKey(trick: Trick) {
  return trick.plays
    .map((play) => `${play.seat}:${play.card.suit}${play.card.rank}`)
    .join("|");
}

export function gameplayMotionKey(game: GameProjection) {
  return [
    game.board.number,
    game.phase,
    game.completedTricks.length,
    cardKey(game.currentTrick),
  ].join(":");
}

export function gameplayMotionEvents(
  previous: GameProjection,
  current: GameProjection,
): GameplayMotionEvent[] {
  if (
    previous.board.number !== current.board.number ||
    current.completedTricks.length < previous.completedTricks.length
  ) {
    return [{ kind: "sync", trick: current.currentTrick }];
  }

  const events: GameplayMotionEvent[] = [];
  if (current.completedTricks.length > previous.completedTricks.length) {
    const completed = current.completedTricks.slice(previous.completedTricks.length);
    completed.forEach((trick, _trickIndex) => {
      const knownPlayCount = _trickIndex === 0 ? previous.currentTrick.plays.length : 0;
      for (
        let _playIndex = knownPlayCount;
        _playIndex < trick.plays.length;
        _playIndex++
      ) {
        const play = trick.plays[_playIndex]!;
        events.push({
          kind: "play",
          trick: { ...trick, plays: trick.plays.slice(0, _playIndex + 1) },
          movingSeat: play.seat,
        });
      }
      events.push({ kind: "winner", trick });
      events.push({ kind: "collect", trick });
    });
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
