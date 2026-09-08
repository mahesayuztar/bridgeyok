import assert from "node:assert/strict";
import test from "node:test";
import { normalizeBoardReplay, replayFrame } from "./board-replay.ts";

const replay = {
  fullDeal: {
    north: [{ suit: "S", rank: "A" }],
    east: [{ suit: "S", rank: "K" }],
    south: [{ suit: "S", rank: "Q" }],
    west: [{ suit: "S", rank: "J" }],
  },
  game: {
    completedTricks: [
      {
        winner: "N",
        plays: [
          { seat: "N", card: { suit: "S", rank: "A" } },
          { seat: "E", card: { suit: "S", rank: "K" } },
          { seat: "S", card: { suit: "S", rank: "Q" } },
          { seat: "W", card: { suit: "S", rank: "J" } },
        ],
      },
    ],
  },
};

test("replay navigation removes played cards and restores all hands when rewound", () => {
  const original = structuredClone(replay);
  assert.equal(replayFrame(replay, 0).hands.N.length, 1);
  const trick = replayFrame(replay, 4);
  assert.equal(trick.trick.winner, "N");
  assert.equal(Object.values(trick.hands).flat().length, 0);
  assert.equal(trick.showResult, false);
  assert.equal(replayFrame(replay, 5).showResult, true);
  assert.equal(Object.values(replayFrame(replay, 5).hands).flat().length, 4);
  assert.equal(replayFrame(replay, 100).showResult, true);
  assert.equal(Object.values(replayFrame(replay, -1).hands).flat().length, 4);
  assert.deepEqual(replay, original);
});

test("passed out and zero-trick claims reach the result without invented plays", () => {
  for (const completedTricks of [null, []]) {
    const board = { ...replay, game: { completedTricks } };
    assert.equal(replayFrame(board, 0).lastStep, 1);
    assert.equal(replayFrame(board, 1).showResult, true);
    assert.equal(replayFrame(board, 1).trick, undefined);
    assert.equal(Object.values(replayFrame(board, 1).hands).flat().length, 4);
  }
});

test("replay input uses shared game normalization and rejects incomplete deals", () => {
  const ranks = [
    "2",
    "3",
    "4",
    "5",
    "6",
    "7",
    "8",
    "9",
    "T",
    "J",
    "Q",
    "K",
    "A",
  ];
  const fullDeal = Object.fromEntries(
    ["north", "east", "south", "west"].map((hand, _index) => [
      hand,
      ranks.map((rank) => ({ suit: ["S", "H", "D", "C"][_index], rank })),
    ]),
  );
  const raw = {
    boardId: "completed-board",
    fullDeal,
    game: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "BOARD_SCORED",
      auction: {
        dealer: "N",
        calls: ["N", "E", "S", "W"].map((seat) => ({
          seat,
          call: { kind: "PASS" },
        })),
        complete: true,
        passedOut: true,
      },
      dummyRevealed: false,
      currentTrick: { leader: "", plays: null },
      completedTricks: null,
      tricksNS: 0,
      tricksEW: 0,
      claimed: false,
      result: {
        passedOut: true,
        tricksNS: 0,
        tricksEW: 0,
        tricksDeclarer: 0,
        vulnerability: "NONE",
        scoreNS: 0,
      },
    },
  };
  const normalized = normalizeBoardReplay(raw);
  assert.deepEqual(normalized.game.completedTricks, []);
  assert.deepEqual(normalized.game.currentTrick, { plays: [] });
  assert.equal(normalized.game.fullDeal.north.length, 13);
  assert.throws(() =>
    normalizeBoardReplay({ ...raw, fullDeal: { ...fullDeal, north: [] } }),
  );
  assert.throws(() =>
    normalizeBoardReplay({
      ...raw,
      fullDeal: { ...fullDeal, north: fullDeal.east },
    }),
  );
  assert.throws(() =>
    normalizeBoardReplay({ ...raw, game: { ...raw.game, phase: "PLAY" } }),
  );
});


test("every played card reconstructs partial trick, hand, turn and dummy visibility", () => {
  for (let _step = 0; _step <= 4; _step++) {
    const frame = replayFrame(replay, _step);
    assert.equal(Object.values(frame.hands).flat().length, 4 - _step);
    assert.equal(frame.currentTrick.plays.length, _step % 4);
    assert.equal(frame.turn, ["N", "E", "S", "W", "N"][_step]);
    assert.equal(frame.dummyRevealed, _step > 0);
    if (_step > 0) assert.equal(frame.trick.plays.length, _step);
  }
  assert.equal(replayFrame(replay, 3).hands.W.length, 1);
  assert.equal(replayFrame(replay, 2).hands.S.length, 1);
});
