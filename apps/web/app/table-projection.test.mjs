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
    viewerParticipantId: "participant-a",
    viewerRole: "OWNER",
    viewerSeat: "N",
    participants: [{ id: "participant-a", nickname: "Mahesa", role: "OWNER" }],
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

test("rejects malformed nested projection items instead of exposing them to renderers", () => {
  const projection = activeProjection();
  projection.game.auction.calls = [null];

  assert.equal(normalizeLiveTableProjection(projection), null);
});

test("normalizes presence snapshots and rejects malformed timestamps", () => {
  const offline = {
    participantId: "participant-1",
    online: false,
    offlineSince: "2026-08-31T10:00:00Z",
    expiresAt: "2026-08-31T10:01:00Z"
  };
  assert.deepEqual(normalizeParticipantPresence(offline), offline);
  assert.deepEqual(normalizePresenceSnapshot({ participants: [offline] }), [offline]);
  assert.equal(normalizeParticipantPresence({ ...offline, expiresAt: "not-a-date" }), null);
  assert.deepEqual(normalizePresenceSnapshot({ participants: null }), []);
});
