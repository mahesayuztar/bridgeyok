import assert from "node:assert/strict";
import test from "node:test";
import { phase6MatchFixtures, phase6SessionFixtures } from "../e2e/fixtures/phase6-session-fixtures.ts";
import { normalizeLiveTableProjection } from "./table-projection.ts";

test("phase 6 deterministic fixtures cover the persistent session states", () => {
  assert.deepEqual(
    Object.values(phase6SessionFixtures).map(table => table.game.phase),
    ["AUCTION", "OPENING_LEAD", "PLAY", "PLAY", "BOARD_SCORED"],
  );
  assert.equal(phase6SessionFixtures.middleTrickDummy.viewerSeat, "S");
  assert.equal(phase6SessionFixtures.middleTrickDefender.viewerSeat, "W");
  assert.equal(phase6MatchFixtures.active.status, "ACTIVE");
  assert.equal(phase6MatchFixtures.complete.status, "COMPLETE");

  for (const table of Object.values(phase6SessionFixtures)) {
    assert.notEqual(normalizeLiveTableProjection(structuredClone(table)), null);
  }
});

test("phase 6 live fixtures preserve recipient raw-frame privacy", () => {
  for (const [name, table] of Object.entries(phase6SessionFixtures)) {
    if (name !== "scored") assert.equal(table.game.fullDeal, undefined, `${name} exposed fullDeal`);
    if (name === "middleTrickDefender") assert.equal(table.game.dummyHand, undefined);
    if (!table.game.dummyRevealed) assert.equal(table.game.dummyHand, undefined, `${name} exposed dummyHand`);
  }

  assert.equal(phase6SessionFixtures.middleTrickDummy.game.completedTricks.length, 1);
  assert.equal(phase6SessionFixtures.middleTrickDefender.game.completedTricks.length, 1);
  assert.equal(phase6SessionFixtures.scored.game.fullDeal.north.length, 13);
  assert.equal(phase6SessionFixtures.scored.game.fullDeal.east.length, 13);
  assert.equal(phase6SessionFixtures.scored.game.fullDeal.south.length, 13);
  assert.equal(phase6SessionFixtures.scored.game.fullDeal.west.length, 13);
});
