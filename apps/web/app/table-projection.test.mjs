import assert from "node:assert/strict";
import test from "node:test";
import { normalizeLiveTableProjection, normalizeParticipantPresence, normalizePresenceSnapshot } from "./table-projection.ts";

function activeProjection() {
  return {
    tableId: "table-a",
    state: "ACTIVE",
    locked: true,
    revision: 1,
    lastSeq: 1,
    boardId: "board-a",
    boardNumber: 1,
    scoreSheet: [],
    pairScoreTotals: [],
    viewerParticipantId: "participant-a",
    viewerRole: "OWNER",
    viewerSeat: "N",
    canRequestUndo: true,
    participants: [{ id: "participant-a", nickname: "Mahesa", role: "OWNER", isBot: false }],
    seats: { N: { participantId: "participant-a", ready: true, controllerEpoch: 1 } },
    game: {
      rulesetVersion: "v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "AUCTION",
      auction: { dealer: "N", turn: "N", calls: null, complete: false, passedOut: false },
      legalCalls: [{ kind: "PASS" }],
      turn: "N",
      dummyRevealed: false,
      currentTrick: { leader: "", plays: null },
      completedTrickCount: 0,
      completedTricks: null,
      tricksNS: 0,
      tricksEW: 0,
      ownHand: null
    }
  };
}

test("normalizes nullable Go collections before table state reaches the UI", () => {
  const table = normalizeLiveTableProjection(activeProjection());

  assert.notEqual(table, null);
  assert.deepEqual(table.game.auction.calls, []);
  assert.deepEqual(table.game.currentTrick.plays, []);
  assert.deepEqual(table.game.completedTricks, []);
  assert.deepEqual(table.game.ownHand, []);
});

test("normalizes durable score rows and rejects non-canonical pair identities", () => {
  const projection = activeProjection();
  const north = { id: "participant-a", nickname: "North", isBot: false };
  const south = { id: "participant-c", nickname: "South", isBot: false };
  const east = { id: "participant-b", nickname: "East", isBot: false };
  const west = { id: "participant-d", nickname: "West", isBot: false };
  const northSouth = { id: "participant-a:participant-c", members: [north, south] };
  const eastWest = { id: "participant-b:participant-d", members: [east, west] };
  projection.scoreSheet = [{
    boardId: "board-a",
    boardNumber: 1,
    result: {
      rulesetVersion: "bridgeyok_duplicate_v1",
      passedOut: true,
      tricksDeclarer: 0,
      tricksNS: 0,
      tricksEW: 0,
      vulnerability: "NONE",
      scoreNS: 0,
    },
    lineup: { seats: { N: north, E: east, S: south, W: west }, northSouth, eastWest },
  }];
  projection.pairScoreTotals = [
    { pair: northSouth, score: 0 },
    { pair: eastWest, score: 0 },
  ];

  const table = normalizeLiveTableProjection(projection);

  assert.notEqual(table, null);
  assert.equal(table.scoreSheet[0].boardId, "board-a");
  projection.scoreSheet[0].lineup.northSouth.id = "seat-dependent";
  assert.equal(normalizeLiveTableProjection(projection), null);
});

test("rejects malformed nested projection items instead of exposing them to renderers", () => {
  const projection = activeProjection();
  projection.game.auction.calls = [null];

  assert.equal(normalizeLiveTableProjection(projection), null);
});

test("rejects completed trick history broader than recipient entitlement", () => {
  const projection = activeProjection();
  projection.game.phase = "PLAY";
  projection.game.auction.contract = {
    level: 1,
    strain: "C",
    doubling: "UNDOUBLED",
    declarer: "N",
  };
  projection.game.completedTrickCount = 2;
  projection.game.completedTricks = [
    {
      plays: [{ seat: "N", card: { suit: "C", rank: "A" } }],
      winner: "N",
    },
    {
      plays: [{ seat: "E", card: { suit: "D", rank: "A" } }],
      winner: "E",
    },
  ];

  assert.equal(normalizeLiveTableProjection(projection), null);
  projection.viewerSeat = "S";
  assert.notEqual(normalizeLiveTableProjection(projection), null);
});

test("normalizes claim and undo consensus state", () => {
  const projection = activeProjection();
  projection.actionRequest = { kind: "CLAIM", requesterSeat: "N", claimTricks: 7, approvedBy: ["E"], canRespond: false };

  const table = normalizeLiveTableProjection(projection);

  assert.notEqual(table, null);
  assert.equal(table.canRequestUndo, true);
  assert.deepEqual(table.actionRequest, projection.actionRequest);
  projection.actionRequest.approvedBy = ["invalid"];
  assert.equal(normalizeLiveTableProjection(projection), null);
});

test("normalizes bot participants and seat assignments", () => {
  const projection = activeProjection();
  projection.participants.push({ id: "bot-west", nickname: "Bot", role: "PARTICIPANT", isBot: true });
  projection.seats.W = { participantId: "bot-west", ready: true, controllerEpoch: 1, isBot: true };

  const table = normalizeLiveTableProjection(projection);

  assert.notEqual(table, null);
  assert.equal(table.participants[1].isBot, true);
  assert.equal(table.seats.W.isBot, true);
});

test("normalizes presence snapshots and rejects malformed timestamps", () => {
  const offline = {
    participantId: "participant-1",
    online: false,
    offlineSince: "2026-08-31T10:00:00Z"
  };
  assert.deepEqual(normalizeParticipantPresence(offline), offline);
  assert.deepEqual(normalizePresenceSnapshot({ participants: [offline] }), [offline]);
  assert.deepEqual(normalizePresenceSnapshot({ participants: null }), []);
});
