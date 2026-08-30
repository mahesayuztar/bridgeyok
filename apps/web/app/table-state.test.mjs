import assert from "node:assert/strict";
import test from "node:test";
import { auctionRows, boardResultLabel, createEmptyTableState, playableHand, reduceTableState, tableOrientation, visualPositionForSeat } from "./table-state.ts";

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

  const issue = { kind: "network", title: "Terputus", detail: "Koneksi terputus.", retryable: true, source: "websocket" };
  const disconnected = reduceTableState(withPending, { type: "connectionLost", issue });
  assert.deepEqual(disconnected.pending, {});
  assert.equal(disconnected.issue, issue);

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

test("controller recovery waits for fresh projection and replacement event", () => {
  const projected = {
    ...tableProjection("table-a", 4),
    viewerSeat: "S",
    seats: { S: { participantId: "viewer", ready: true, controllerEpoch: 1 } }
  };
  const entered = reduceTableState(createEmptyTableState(), { type: "enter", table: projected });
  const pending = reduceTableState(entered, { type: "pending", requestId: "stale-command", commandName: "game.make_call" });
  const issue = { kind: "conflict", title: "Kendali stale", detail: "Selaraskan.", retryable: true, action: "resync", source: "websocket" };
  const syncing = reduceTableState(pending, { type: "conflict", issue });
  assert.equal(syncing.controllerState, "resyncing");
  assert.deepEqual(syncing.pending, {});

  const freshProjection = {
    ...projected,
    revision: 5,
    lastSeq: 5,
    seats: { S: { participantId: "viewer", ready: true, controllerEpoch: 2 } }
  };
  const fresh = reduceTableState(syncing, { type: "snapshot", tableId: "table-a", seq: 5, table: freshProjection });
  assert.equal(fresh.controllerState, "readyToTakeover");
  assert.equal(fresh.issue.action, "takeover");
  const repeatedSnapshot = reduceTableState(fresh, { type: "snapshot", tableId: "table-a", seq: 5, table: freshProjection });
  assert.equal(repeatedSnapshot.controllerState, "readyToTakeover");

  const takeover = reduceTableState(fresh, { type: "pending", requestId: "takeover", commandName: "table.takeover" });
  assert.equal(takeover.controllerState, "takeoverPending");
  const replaced = reduceTableState(takeover, {
    type: "event",
    tableId: "table-a",
    seq: 6,
    eventType: "CONTROLLER_REPLACED",
    table: { ...freshProjection, revision: 6, lastSeq: 6, seats: { S: { participantId: "viewer", ready: true, controllerEpoch: 3 } } }
  });
  assert.equal(replaced.controllerState, "current");
  assert.equal(replaced.notice, "Kendali sudah berpindah ke perangkat ini.");
});

test("table orientation keeps the viewer at the bottom", () => {
  const orientation = tableOrientation("N");
  assert.deepEqual(orientation, { top: "S", right: "W", bottom: "N", left: "E" });
  assert.equal(visualPositionForSeat(orientation, "E"), "left");
});

test("auction rows preserve W N E S columns and dealer offset", () => {
  const rows = auctionRows("E", [
    { seat: "E", call: { kind: "BID", level: 1, strain: "H" } },
    { seat: "S", call: { kind: "PASS" } },
    { seat: "W", call: { kind: "BID", level: 1, strain: "S" } }
  ]);
  assert.deepEqual(rows, [
    { E: { seat: "E", call: { kind: "BID", level: 1, strain: "H" } }, S: { seat: "S", call: { kind: "PASS" } } },
    { W: { seat: "W", call: { kind: "BID", level: 1, strain: "S" } } }
  ]);
  assert.deepEqual(auctionRows("N", null), [{}]);
});

test("board result label describes exact, overtrick, and undertrick", () => {
  const base = {
    passedOut: false,
    contract: { level: 4, strain: "H", doubling: "UNDOUBLED", declarer: "N" },
    tricksDeclarer: 10,
    tricksNS: 10,
    tricksEW: 3,
    vulnerability: "NONE",
    scoreNS: 420
  };
  assert.equal(boardResultLabel(base), "Tepat kontrak");
  assert.equal(boardResultLabel({ ...base, tricksDeclarer: 11 }), "+1");
  assert.equal(boardResultLabel({ ...base, tricksDeclarer: 8 }), "-2");
});
