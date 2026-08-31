import assert from "node:assert/strict";
import test from "node:test";
import { auctionRows, boardResultLabel, createEmptyTableState, playableHand, projectedTableState, reduceTableState, tableOrientation, visualPositionForSeat } from "./table-state.ts";

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
  const withPending = reduceTableState(first, { type: "pending", requestId: "request-1", commandName: "table.lock", payload: {} });
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
  const pending = reduceTableState(initial, { type: "pending", requestId: "request-1", commandName: "table.lock", payload: {} });
  const projected = tableProjection("table-a", 4);
  const next = reduceTableState(pending, { type: "event", tableId: "table-a", seq: 2, table: projected });
  assert.equal(next.table, projected);
  assert.equal(next.lastSeenSeq, 4);
  assert.equal(next.pending["request-1"].name, "table.lock");
});

test("table reducer projects a call until ack and authoritative event converge", () => {
  const table = {
    ...tableProjection("table-a", 4),
    state: "ACTIVE",
    viewerSeat: "N",
    game: {
      rulesetVersion: "v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "AUCTION",
      auction: { dealer: "N", turn: "N", calls: [], complete: false, passedOut: false },
      legalCalls: [{ kind: "PASS" }],
      turn: "N",
      dummyRevealed: false,
      currentTrick: { plays: [] },
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: [],
    },
  };
  const entered = reduceTableState(createEmptyTableState(), { type: "enter", table });
  const pending = reduceTableState(entered, { type: "pending", requestId: "call", commandName: "game.make_call", payload: { call: { kind: "PASS" } } });
  assert.deepEqual(projectedTableState(pending).game.auction.calls, [{ seat: "N", call: { kind: "PASS" } }]);
  const accepted = reduceTableState(pending, { type: "ack", requestId: "call", revision: 5, seq: 5 });
  assert.equal(accepted.pending.call.status, "accepted");
  const authoritative = {
    ...table,
    revision: 5,
    lastSeq: 5,
    game: {
      ...table.game,
      turn: "E",
      auction: { ...table.game.auction, turn: "E", calls: [{ seat: "N", call: { kind: "PASS" } }] },
    },
  };
  const settled = reduceTableState(accepted, { type: "event", tableId: "table-a", seq: 5, table: authoritative });
  assert.deepEqual(settled.pending, {});
  assert.equal(projectedTableState(settled).game.auction.calls.length, 1);
});

test("table reducer tracks presence and removes departed participants", () => {
  const projected = {
    ...tableProjection("table-a", 1),
    participants: [
      { id: "owner", nickname: "Owner", role: "OWNER" },
      { id: "guest", nickname: "Guest", role: "PARTICIPANT" }
    ]
  };
  const entered = reduceTableState(createEmptyTableState(), { type: "enter", table: projected });
  const presence = reduceTableState(entered, {
    type: "presenceSnapshot",
    tableId: "table-a",
    participants: [
      { participantId: "owner", online: true },
      { participantId: "guest", online: false, offlineSince: "2026-08-31T10:00:00Z", expiresAt: "2026-08-31T10:01:00Z" },
      { participantId: "unknown", online: true }
    ]
  });
  assert.deepEqual(Object.keys(presence.presence).sort(), ["guest", "owner"]);
  const online = reduceTableState(presence, {
    type: "presenceChanged",
    tableId: "table-a",
    participant: { participantId: "guest", online: true }
  });
  assert.equal(online.presence.guest.online, true);

  const afterTimeout = reduceTableState(online, {
    type: "event",
    tableId: "table-a",
    seq: 2,
    eventType: "PARTICIPANT_TIMED_OUT",
    table: { ...projected, revision: 2, lastSeq: 2, participants: projected.participants.slice(0, 1) }
  });
  assert.deepEqual(Object.keys(afterTimeout.presence), ["owner"]);
  assert.equal(afterTimeout.notice, "Pemain offline sudah dikeluarkan dari meja.");
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
  const pending = reduceTableState(entered, { type: "pending", requestId: "stale-command", commandName: "game.make_call", payload: {} });
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

  const takeover = reduceTableState(fresh, { type: "pending", requestId: "takeover", commandName: "table.takeover", payload: {} });
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
