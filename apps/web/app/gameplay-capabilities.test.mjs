import assert from "node:assert/strict";
import test from "node:test";
import { canSendTableCommand } from "./gameplay-capabilities.ts";

function activeTable() {
  return {
    tableId: "table-a",
    state: "ACTIVE",
    locked: true,
    revision: 4,
    lastSeq: 4,
    boardNumber: 1,
    viewerParticipantId: "owner",
    viewerRole: "OWNER",
    viewerSeat: "N",
    participants: [{ id: "owner", nickname: "Owner", role: "OWNER" }],
    seats: { N: { participantId: "owner", ready: true, controllerEpoch: 1 } },
    canRequestUndo: false,
    game: {
      rulesetVersion: "v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "AUCTION",
      auction: { dealer: "N", turn: "N", calls: [], complete: false, passedOut: false },
      legalCalls: [{ kind: "PASS" }, { kind: "BID", level: 1, strain: "C" }],
      turn: "N",
      dummyRevealed: false,
      currentTrick: { plays: [] },
      completedTrickCount: 0,
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: [],
    },
  };
}

function context(table = activeTable()) {
  return { table, connected: true, controllerState: "current", hasPendingCommand: false };
}

test("command capability rejects stale, offline, and concurrent actions", () => {
  assert.equal(canSendTableCommand({ ...context(), connected: false }, "game.make_call", { call: { kind: "PASS" } }), false);
  assert.equal(canSendTableCommand({ ...context(), controllerState: "mirror" }, "game.make_call", { call: { kind: "PASS" } }), false);
  assert.equal(canSendTableCommand({ ...context(), controllerState: "resyncing" }, "game.make_call", { call: { kind: "PASS" } }), false);
  assert.equal(canSendTableCommand({ ...context(), hasPendingCommand: true }, "game.make_call", { call: { kind: "PASS" } }), false);
});

test("auction capability follows turn and legal call projection", () => {
  assert.equal(canSendTableCommand(context(), "game.make_call", { call: { kind: "PASS" } }), true);
  assert.equal(canSendTableCommand(context(), "game.make_call", { call: { kind: "DOUBLE" } }), false);
  assert.equal(canSendTableCommand(context({ ...activeTable(), game: { ...activeTable().game, turn: "E" } }), "game.make_call", { call: { kind: "PASS" } }), false);
});

test("play capability enforces card ownership and follow suit", () => {
  const table = activeTable();
  table.game = {
    ...table.game,
    phase: "PLAY",
    currentTrick: { leader: "E", plays: [{ seat: "E", card: { suit: "H", rank: "K" } }] },
    ownHand: [{ suit: "H", rank: "A" }, { suit: "S", rank: "A" }],
  };
  assert.equal(canSendTableCommand(context(table), "game.play_card", { card: { suit: "H", rank: "A" } }), true);
  assert.equal(canSendTableCommand(context(table), "game.play_card", { card: { suit: "S", rank: "A" } }), false);
});

test("automatic takeover is only available after a completed resync", () => {
  assert.equal(canSendTableCommand({ ...context(), controllerState: "readyToTakeover" }, "table.takeover"), true);
  assert.equal(canSendTableCommand(context(), "table.takeover"), false);
});

test("table lifecycle commands follow owner, readiness, and phase", () => {
  const table = activeTable();
  table.state = "WAITING";
  table.game = undefined;
  table.seats = {
    N: { participantId: "owner", ready: true, controllerEpoch: 1 },
    E: { participantId: "east", ready: true, controllerEpoch: 1 },
    S: { participantId: "south", ready: true, controllerEpoch: 1 },
    W: { participantId: "west", ready: true, controllerEpoch: 1 },
  };
  assert.equal(canSendTableCommand(context(table), "table.start_game"), true);
  assert.equal(canSendTableCommand(context(table), "table.finish"), true);
  assert.equal(canSendTableCommand(context(activeTable()), "table.finish"), false);
  assert.equal(canSendTableCommand(context(table), "table.leave"), true);
  assert.equal(canSendTableCommand(context(activeTable()), "table.leave"), true);
  assert.equal(canSendTableCommand(context({ ...table, state: "FINISHED" }), "table.leave"), false);
});

test("consensus capabilities expose only the projected responder", () => {
  const table = {
    ...activeTable(),
    actionRequest: {
      kind: "CLAIM",
      requesterSeat: "E",
      claimTricks: 4,
      approvedBy: [],
      canRespond: true,
    },
  };
  assert.equal(canSendTableCommand(context(table), "game.respond_claim", { accepted: true }), true);
  assert.equal(canSendTableCommand(context(table), "game.respond_undo", { accepted: true }), false);
  assert.equal(canSendTableCommand(context(table), "game.make_call", { call: { kind: "PASS" } }), false);
});
