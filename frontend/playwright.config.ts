import { defineConfig, devices } from "@playwright/test";
import { DEFAULT_FRONTEND_PORT, TIMEOUTS } from "../scripts/lib.mjs";

const E2E_BASE_URL =
  process.env.E2E_BASE_URL ??
  `http://localhost:${process.env.FRONTEND_PORT ?? DEFAULT_FRONTEND_PORT}`;

/**
 * Playwright E2E Test Configuration
 *
 * Tests run against E2E_BASE_URL (dev server; set FRONTEND_PORT to move it).
 * Backend API should run on API_BASE_URL / CARO_HTTP_PORT (default 5207).
 */
export default defineConfig({
  testDir: "./e2e",
  // The backend caps concurrent games at 4 and tests create one game each;
  // serial execution with per-test cleanup keeps the suite under that cap.
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: "html",

  use: {
    baseURL: E2E_BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],

  // Start dev server before running tests
  webServer: {
    command: "npm run dev",
    url: E2E_BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: TIMEOUTS.webServerTimeoutMs,
  },
});
