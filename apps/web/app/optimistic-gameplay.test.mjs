import assert from "node:assert/strict";
import test from "node:test";
import {
  acknowledgePendingCommand,
  createPendingTableCommand,
  projectOptimisticTable,
  reconcilePendingCommands,
} from "./optimistic-gameplay.ts";

function auctionTable() {
  return {
    tableId: "table-a",
    state: "ACTIVE",
    locked: true,
    revision: 4,
    lastSeq: 4,
    boardNumber: 1,
    viewerParticipantId: "viewer",
    viewerRole: "OWNER",
    viewerSeat: "N",
    participants: [],
    seats: {},
    canRequestUndo: false,
    game: {
      rulesetVersion: "v1",
      board: { number: 1, dealer: "N", vulnerability: "NONE" },
      phase: "AUCTION",
      auction: {
        dealer: "N",
        turn: "N",
        calls: [],
        complete: false,
        passedOut: false,
      },
      legalCalls: [
        { kind: "PASS" },
        { kind: "BID", level: 1, strain: "C" },
      ],
      turn: "N",
      dummyRevealed: false,
      currentTrick: { plays: [] },
      completedTricks: [],
      tricksNS: 0,
      tricksEW: 0,
      ownHand: [],
    },
  };
}

function playTable() {
  return {
    ...auctionTable(),
    revision: 8,
    lastSeq: 8,
    game: {
      ...auctionTable().game,
      phase: "PLAY",
      auction: {
        ...auctionTable().game.auction,
        complete: true,
        contract: {
          level: 1,
          strain: "S",
          doubling: "UNDOUBLED",
          declarer: "N",
        },
      },
      legalCalls: [],
      ownHand: [
        { suit: "H", rank: "A" },
        { suit: "S", rank: "2" },
      ],
    },
  };
}

test("optimistic call appears immediately and settles without duplication", () => {
  const table = auctionTable();
  const command = createPendingTableCommand(table, "request-call", "game.make_call", {
    call: { kind: "BID", level: 1, strain: "C" },
  });
  const pending = { [command.requestId]: command };
  const projected = projectOptimisticTable(table, pending);
  assert.deepEqual(projected.game.auction.calls, [
    { seat: "N", call: { kind: "BID", level: 1, strain: "C" } },
  ]);
  assert.equal(projected.game.turn, "E");
  assert.equal(table.game.auction.calls.length, 0);

  const accepted = acknowledgePendingCommand(pending, "request-call", 5, 5, 4);
  assert.equal(accepted["request-call"].status, "accepted");
  const authoritative = {
    ...table,
    revision: 5,
    lastSeq: 5,
    game: {
      ...table.game,
      turn: "E",
      auction: {
        ...table.game.auction,
        turn: "E",
        calls: projected.game.auction.calls,
      },
    },
  };
  assert.deepEqual(reconcilePendingCommands(accepted, authoritative), {});
  assert.equal(projectOptimisticTable(authoritative, {}).game.auction.calls.length, 1);
});

test("event before ack settles a visible optimistic call", () => {
  const table = auctionTable();
  const command = createPendingTableCommand(table, "request-call", "game.make_call", {
    call: { kind: "PASS" },
  });
  const authoritative = {
    ...table,
    revision: 5,
    lastSeq: 5,
    game: {
      ...table.game,
      auction: {
        ...table.game.auction,
        calls: [{ seat: "N", call: { kind: "PASS" } }],
      },
    },
  };
  assert.deepEqual(
    reconcilePendingCommands({ [command.requestId]: command }, authoritative),
    {},
  );
  assert.deepEqual(
    acknowledgePendingCommand({}, "request-call", 5, 5, 5),
    {},
  );
});

test("optimistic play removes its source card and rollback restores authority", () => {
  const table = playTable();
  const command = createPendingTableCommand(table, "request-play", "game.play_card", {
    card: { suit: "H", rank: "A" },
  });
  const pending = { [command.requestId]: command };
  const projected = projectOptimisticTable(table, pending);
  assert.deepEqual(projected.game.ownHand, [{ suit: "S", rank: "2" }]);
  assert.deepEqual(projected.game.currentTrick.plays, [
    { seat: "N", card: { suit: "H", rank: "A" } },
  ]);
  assert.deepEqual(projectOptimisticTable(table, {}), table);
});

test("unrelated revisions retain only operations that remain legal", () => {
  const table = auctionTable();
  const command = createPendingTableCommand(table, "request-call", "game.make_call", {
    call: { kind: "PASS" },
  });
  const stillLegal = { ...table, revision: 5, lastSeq: 5 };
  assert.deepEqual(
    reconcilePendingCommands({ [command.requestId]: command }, stillLegal),
    { [command.requestId]: command },
  );
  const remoteCall = {
    ...stillLegal,
    game: {
      ...table.game,
      turn: "E",
      auction: {
        ...table.game.auction,
        turn: "E",
        calls: [{ seat: "N", call: { kind: "BID", level: 1, strain: "C" } }],
      },
    },
  };
  assert.deepEqual(
    reconcilePendingCommands({ [command.requestId]: command }, remoteCall),
    {},
  );
});
