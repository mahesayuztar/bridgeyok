import { defineConfig, devices } from "@playwright/test";

const baseURL = "http://localhost:3100";
const apiURL = "http://localhost:8180";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 120_000,
  expect: { timeout: 10_000 },
  reporter: process.env.CI ? "github" : "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    trace: "retain-on-failure"
  },
  webServer: [
    {
      command: "go run ../api/cmd/api",
      url: `${apiURL}/health/ready`,
      timeout: 120_000,
      reuseExistingServer: false,
      env: {
        ...process.env,
        ALLOWED_ORIGINS: baseURL,
        API_HOST: "127.0.0.1",
        APP_ENV: "test",
        AUTH_SECRET: "phase3-browser-gate-secret-at-least-32-characters",
        PORT: "8180"
      }
    },
    {
      command: "corepack pnpm exec next dev --port 3100",
      url: baseURL,
      timeout: 120_000,
      reuseExistingServer: false,
      env: {
        ...process.env,
        API_BASE_URL: apiURL,
        NEXT_PUBLIC_API_BASE_URL: apiURL,
        NEXT_DIST_DIR: ".next-e2e",
        PORT: "3100"
      }
    }
  ]
});
