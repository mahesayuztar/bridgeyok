import assert from "node:assert/strict";
import test from "node:test";
import {
  callKey,
  callLabel,
  cardKey,
  contractLabel,
  contractSummaryLabel,
  groupCardsForContract,
  organizeCardsForContract,
  suitOrderForContract,
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

test("contract presentation keeps only contract, doubling, and declarer seat", () => {
  const contract = {
    level: 4,
    strain: "H",
    doubling: "DOUBLED",
    declarer: "N",
  };
  assert.equal(contractLabel(contract), "4♥ X");
  assert.equal(contractSummaryLabel(contract), "4♥ X N");
  assert.equal(contractLabel(undefined), "Belum ada kontrak");
  assert.equal(contractSummaryLabel(undefined), "Belum ada kontrak");
  assert.equal(
    contractSummaryLabel({ ...contract, strain: "NT", doubling: "REDOUBLED" }),
    "4NT XX N",
  );
});

test("contract organizes viewer and dummy suit groups without mutating projection", () => {
  const cards = [
    { suit: "D", rank: "2" },
    { suit: "S", rank: "A" },
    { suit: "C", rank: "3" },
    { suit: "H", rank: "K" },
    { suit: "S", rank: "Q" },
  ];

  assert.deepEqual(
    organizeCardsForContract(cards).map((card) => card.suit),
    ["S", "S", "H", "D", "C"],
  );
  assert.deepEqual(
    organizeCardsForContract(cards, "S").map((card) => card.suit),
    ["S", "S", "H", "D", "C"],
  );
  assert.deepEqual(
    organizeCardsForContract(cards, "H").map((card) => card.suit),
    ["H", "S", "S", "C", "D"],
  );
  assert.deepEqual(
    organizeCardsForContract(cards, "C").map((card) => card.suit),
    ["C", "H", "S", "S", "D"],
  );
  assert.deepEqual(
    organizeCardsForContract(cards, "D").map((card) => card.suit),
    ["D", "S", "S", "H", "C"],
  );
  assert.deepEqual(cards.map((card) => card.suit), ["D", "S", "C", "H", "S"]);
  assert.deepEqual(suitOrderForContract("D"), ["D", "S", "H", "C"]);
  assert.deepEqual(
    groupCardsForContract(
      [
        { suit: "S", rank: "A" },
        { suit: "H", rank: "K" },
        { suit: "D", rank: "Q" },
      ],
      "C",
    ).map((cardGroup) => cardGroup.suit),
    ["H", "S", "D"],
  );
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
