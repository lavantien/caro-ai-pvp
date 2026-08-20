import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright E2E Test Configuration
 *
 * Tests run against http://localhost:5173 (dev server)
 * Backend API should run on http://localhost:5207
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
    baseURL: "http://localhost:5173",
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
    url: "http://localhost:5173",
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
});
