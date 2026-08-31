import assert from "node:assert/strict";
import test from "node:test";
import {
  callKey,
  callLabel,
  cardKey,
  contractLabel,
  sortCardsDescending,
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

test("contract presentation preserves declarer and doubling options", () => {
  const contract = {
    level: 4,
    strain: "H",
    doubling: "DOUBLED",
    declarer: "N",
  };
  assert.equal(contractLabel(contract), "4♥ X oleh N");
  assert.equal(contractLabel(contract, false), "4♥ X");
  assert.equal(contractLabel(undefined), "Belum ada kontrak");
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
