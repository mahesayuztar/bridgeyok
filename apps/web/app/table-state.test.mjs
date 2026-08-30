import assert from "node:assert/strict";
import test from "node:test";
import { createEmptyTableState, playableHand, reduceTableState } from "./table-state.ts";

function tableProjection(tableId, lastSeq = 0) {
  return {
    tableId,
    state: "WAITING",
    locked: false,
    revision: lastSeq,
    lastSeq,
    boardNumber: 0,
    viewerParticipantId: "viewer",
    viewerRole: "PARTICIPANT",
    participants: [],
    seats: {}
  };
}

test("table reducer clears private state and ignores stale rooms", () => {
  const first = reduceTableState(createEmptyTableState(), { type: "enter", table: tableProjection("table-a", 2) });
  const withPending = reduceTableState(first, { type: "pending", requestId: "request-1", commandName: "table.lock" });
  const staleRoom = reduceTableState(withPending, {
    type: "event",
    tableId: "table-b",
    seq: 3,
    table: tableProjection("table-b", 3)
  });
  assert.equal(staleRoom, withPending);

  const duplicate = reduceTableState(withPending, {
    type: "event",
    tableId: "table-a",
    seq: 2,
    table: tableProjection("table-a", 2)
  });
  assert.equal(duplicate, withPending);

  const disconnected = reduceTableState(withPending, { type: "connectionLost", message: "Koneksi terputus." });
  assert.deepEqual(disconnected.pending, {});
  assert.equal(disconnected.message, "Koneksi terputus.");

  const cleared = reduceTableState(withPending, { type: "clear" });
  assert.deepEqual(cleared, createEmptyTableState());
});

test("table reducer converges to projected event sequence", () => {
  const initial = reduceTableState(createEmptyTableState(), { type: "enter", table: tableProjection("table-a", 1) });
  const pending = reduceTableState(initial, { type: "pending", requestId: "request-1", commandName: "table.lock" });
  const projected = tableProjection("table-a", 4);
  const next = reduceTableState(pending, { type: "event", tableId: "table-a", seq: 2, table: projected });
  assert.equal(next.table, projected);
  assert.equal(next.lastSeenSeq, 4);
  assert.deepEqual(next.pending, {});
});

test("playable hand supports declarer dummy control and follow suit", () => {
  const table = {
    ...tableProjection("table-a", 8),
    state: "ACTIVE",
    viewerSeat: "N",
    game: {
      rulesetVersion: "v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "PLAY",
      auction: {
        dealer: "N",
        calls: [],
        complete: true,
        passedOut: false,
        contract: { level: 1, strain: "S", doubling: "UNDOUBLED", declarer: "N" }
      },
      turn: "S",
      dummyRevealed: true,
      currentTrick: { leader: "E", plays: [{ seat: "E", card: { suit: "H", rank: "A" } }] },
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: [{ suit: "C", rank: "2" }],
      dummyHand: [
        { suit: "H", rank: "2" },
        { suit: "S", rank: "A" }
      ]
    }
  };
  assert.deepEqual(playableHand(table), { hand: [{ suit: "H", rank: "2" }], source: "dummy" });
});
