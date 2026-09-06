import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  testMatch: "trick-history-layout.spec.ts",
  use: { browserName: "chromium" },
});
