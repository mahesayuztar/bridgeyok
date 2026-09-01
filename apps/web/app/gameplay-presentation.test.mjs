import assert from "node:assert/strict";
import test from "node:test";
import {
  callKey,
  callLabel,
  cardKey,
  contractLabel,
  contractSummaryLabel,
  participantNameForSeat,
  sortCardsDescending,
  viewerTrickCounts,
} from "./table/gameplay-presentation.ts";

test("gameplay presentation keeps bridge labels stable", () => {
  assert.equal(callLabel({ kind: "PASS" }), "Pass");
  assert.equal(callLabel({ kind: "DOUBLE" }), "X");
  assert.equal(callLabel({ kind: "REDOUBLE" }), "XX");
  assert.equal(callLabel({ kind: "BID", level: 4, strain: "S" }), "4♠");
  assert.equal(callLabel({ kind: "BID", level: 3, strain: "NT" }), "3NT");
  assert.equal(callKey({ kind: "BID", level: 4, strain: "S" }), "BID:4:S");
  assert.equal(cardKey({ suit: "H", rank: "T" }), "HT");
});

test("contract presentation preserves declarer identity and doubling", () => {
  const contract = {
    level: 4,
    strain: "H",
    doubling: "DOUBLED",
    declarer: "N",
  };
  assert.equal(contractLabel(contract), "4♥ X");
  assert.equal(contractSummaryLabel(contract, "Mahesa"), "4♥ X · N · Mahesa");
  assert.equal(contractSummaryLabel(contract), "4♥ X · N");
  assert.equal(contractLabel(undefined), "Belum ada kontrak");
  assert.equal(contractSummaryLabel(undefined), "Belum ada kontrak");
  assert.equal(
    contractSummaryLabel({ ...contract, strain: "NT", doubling: "REDOUBLED" }, "Nama pemain yang panjang"),
    "4NT XX · N · Nama pemain yang panjang",
  );
});

test("contract presentation resolves the declarer name from seat assignments", () => {
  assert.equal(
    participantNameForSeat(
      {
        seats: { N: { participantId: "north" } },
        participants: [{ id: "north", nickname: "Nara" }],
      },
      "N",
    ),
    "Nara",
  );
  assert.equal(
    participantNameForSeat({ seats: {}, participants: [] }, "E"),
    undefined,
  );
});

test("dummy presentation sorts ranks descending without mutating the hand", () => {
  const cards = [
    { suit: "S", rank: "2" },
    { suit: "S", rank: "A" },
    { suit: "S", rank: "T" },
  ];
  assert.deepEqual(sortCardsDescending(cards), [
    { suit: "S", rank: "A" },
    { suit: "S", rank: "T" },
    { suit: "S", rank: "2" },
  ]);
  assert.deepEqual(cards, [
    { suit: "S", rank: "2" },
    { suit: "S", rank: "A" },
    { suit: "S", rank: "T" },
  ]);
});

test("trick counts follow the viewer partnership", () => {
  const table = {
    viewerSeat: "N",
    game: { tricksNS: 8, tricksEW: 5 },
  };
  assert.deepEqual(viewerTrickCounts(table), {
    viewerPartnership: "NS",
    won: 8,
    lost: 5,
  });
  assert.deepEqual(viewerTrickCounts({ ...table, viewerSeat: "S" }), {
    viewerPartnership: "NS",
    won: 8,
    lost: 5,
  });
  assert.deepEqual(viewerTrickCounts({ ...table, viewerSeat: "E" }), {
    viewerPartnership: "EW",
    won: 5,
    lost: 8,
  });
  assert.deepEqual(viewerTrickCounts({ ...table, viewerSeat: "W" }), {
    viewerPartnership: "EW",
    won: 5,
    lost: 8,
  });
});

test("trick counts retain an NS-EW fallback without a viewer seat", () => {
  assert.deepEqual(
    viewerTrickCounts({ game: { tricksNS: 13, tricksEW: 0 } }),
    { viewerPartnership: null, won: 13, lost: 0 },
  );
  assert.equal(viewerTrickCounts({}), null);
});
