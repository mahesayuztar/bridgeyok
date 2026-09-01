import assert from "node:assert/strict";
import test from "node:test";
import {
  GAMEPLAY_MOTION_DURATION,
  gameplayMotionEvents,
  shouldSkipCompletedTrickPause,
} from "./gameplay-motion.ts";
import {
  shouldPlayTurnCue,
  TURN_CUE_DURATION_SECONDS,
  TURN_CUE_PEAK_GAIN,
  turnCueState,
} from "./turn-cue.ts";

function game({
  boardNumber = 1,
  phase = "PLAY",
  currentTrick = { plays: [] },
  completedTricks = [],
} = {}) {
  return {
    board: { number: boardNumber, dealer: "N", vulnerability: "NONE" },
    phase,
    currentTrick,
    completedTricks,
  };
}

const plays = [
  { seat: "N", card: { suit: "S", rank: "A" } },
  { seat: "E", card: { suit: "S", rank: "2" } },
  { seat: "S", card: { suit: "S", rank: "K" } },
  { seat: "W", card: { suit: "S", rank: "3" } },
];

test("gameplay motion queues newly played cards in order", () => {
  const events = gameplayMotionEvents(
    game(),
    game({ currentTrick: { leader: "N", plays: plays.slice(0, 2) } }),
  );
  assert.deepEqual(events.map((event) => event.kind), ["play", "play"]);
  assert.deepEqual(events.map((event) => event.movingSeat), ["N", "E"]);
  assert.equal(events[1].trick.plays.length, 2);
});

test("gameplay motion preserves the winner pause before collection", () => {
  const completedTrick = { leader: "N", plays, winner: "N" };
  const events = gameplayMotionEvents(
    game({ currentTrick: { leader: "N", plays: plays.slice(0, 3) } }),
    game({ completedTricks: [completedTrick] }),
  );
  assert.deepEqual(events.map((event) => event.kind), [
    "play",
    "winner",
    "collect",
  ]);
  assert.equal(events[1].trick.winner, "N");
  assert.equal(GAMEPLAY_MOTION_DURATION.winnerPause, 3_200);
});

test("a lead in the next trick skips the completed trick pause", () => {
  const completedTrick = { leader: "N", plays, winner: "N" };
  assert.equal(
    shouldSkipCompletedTrickPause(
      game({ completedTricks: [completedTrick] }),
      game({
        completedTricks: [completedTrick],
        currentTrick: { leader: "N", plays: plays.slice(0, 1) },
      }),
    ),
    true,
  );
  assert.equal(
    shouldSkipCompletedTrickPause(
      game(),
      game({ currentTrick: { leader: "N", plays: plays.slice(0, 1) } }),
    ),
    false,
  );
});

test("gameplay motion syncs instead of replaying on board replacement", () => {
  const events = gameplayMotionEvents(
    game({ boardNumber: 1, currentTrick: { plays: plays.slice(0, 2) } }),
    game({ boardNumber: 2, currentTrick: { plays: [] } }),
  );
  assert.deepEqual(events, [{ kind: "sync", trick: { plays: [] } }]);
});

function cueState({ turn, viewerSeat = "N", revision = 1, phase = "PLAY" }) {
  return {
    tableId: "table-1",
    boardNumber: 1,
    phase,
    turn,
    viewerTurn: turn === viewerSeat,
    revision,
  };
}

test("turn cue fires only when authority transitions to the viewer", () => {
  assert.equal(
    shouldPlayTurnCue(
      cueState({ turn: "W" }),
      cueState({ turn: "N", revision: 2 }),
    ),
    true,
  );
  assert.equal(
    shouldPlayTurnCue(
      cueState({ turn: "N" }),
      cueState({ turn: "N", revision: 2 }),
    ),
    false,
  );
});

test("turn cue ignores initial snapshots, rerenders, and scored boards", () => {
  const viewerTurn = cueState({ turn: "N" });
  assert.equal(shouldPlayTurnCue(null, viewerTurn), false);
  assert.equal(shouldPlayTurnCue(viewerTurn, viewerTurn), false);
  assert.equal(
    shouldPlayTurnCue(
      cueState({ turn: "W" }),
      cueState({ turn: "N", revision: 2, phase: "BOARD_SCORED" }),
    ),
    false,
  );
});

test("turn cue assigns dummy control to declarer and never to dummy", () => {
  const table = (viewerSeat, turn, revision) => ({
    tableId: "table-1",
    revision,
    viewerSeat,
    game: {
      board: { number: 1 },
      phase: "PLAY",
      turn,
      auction: {
        contract: { declarer: "N" },
      },
    },
  });
  assert.equal(
    shouldPlayTurnCue(
      turnCueState(table("N", "E", 1)),
      turnCueState(table("N", "S", 2)),
    ),
    true,
  );
  assert.equal(
    shouldPlayTurnCue(
      turnCueState(table("S", "E", 1)),
      turnCueState(table("S", "S", 2)),
    ),
    false,
  );
});

test("turn cue remains clearly audible without clipping", () => {
  assert.equal(TURN_CUE_PEAK_GAIN, 0.18);
  assert.equal(TURN_CUE_DURATION_SECONDS, 0.22);
  assert.ok(TURN_CUE_PEAK_GAIN < 1);
});
