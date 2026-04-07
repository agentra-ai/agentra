import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  retries: 0,
  use: {
    baseURL:
      process.env.PLAYWRIGHT_BASE_URL ??
      process.env.NEXT_PUBLIC_SITE_URL ??
      process.env.FRONTEND_ORIGIN ??
      process.env.AGENTRA_APP_URL ??
      "http://localhost:3000",
    headless: true,
  },
  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],
  // Don't auto-start servers — they must be running already
  // This avoids complexity and port conflicts during testing
});
