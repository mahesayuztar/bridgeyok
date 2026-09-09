import assert from "node:assert/strict";
import test from "node:test";
import { safeReturnPath } from "./account-types.ts";

test("auth return paths keep app destinations and reject external or reverse-auth loops", () => {
  for (const path of ["/play", "/friends", "/settings", "/lobby?invite=ABC", "/table/123"]) assert.equal(safeReturnPath(path), path);
  for (const path of [undefined, "https://evil.test", "//evil.test", "/\\evil.test", "/login", "/signup", "/", "/deals", "/team-match", "/play\n"]) assert.equal(safeReturnPath(path), "/play");
});
