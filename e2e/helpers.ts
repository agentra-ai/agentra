import { type Page } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const DEFAULT_E2E_NAME = "E2E User";
const E2E_WORKER_ID = process.env.TEST_PARALLEL_INDEX ?? "0";
const DEFAULT_E2E_EMAIL = `e2e+worker-${E2E_WORKER_ID}@example.test`;
const DEFAULT_E2E_WORKSPACE = `e2e-workspace-${E2E_WORKER_ID}`;

/**
 * Log in as the default E2E user and ensure the workspace exists first.
 * Authenticates via API (send-code → DB read → verify-code), then injects
 * the token into localStorage so the browser session is authenticated.
 */
export async function loginAsDefault(page: Page) {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  const workspace = await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);

  const token = api.getToken();
  await page.addInitScript(({ authToken, workspaceId }) => {
    localStorage.setItem("agentra_token", authToken);
    localStorage.setItem("agentra_workspace_id", workspaceId);
  }, { authToken: token, workspaceId: workspace.id });
  await page.goto("/issues", { waitUntil: "domcontentloaded" });
  await page.waitForURL("**/issues", { timeout: 10000 });
  await page
    .getByRole("link", { name: "Issues", exact: true })
    .waitFor({ state: "visible", timeout: 30000 });
}

/**
 * Create a TestApiClient logged in as the default E2E user.
 * Call api.cleanup() in afterEach to remove test data created during the test.
 */
export async function createTestApi(): Promise<TestApiClient> {
  const api = new TestApiClient();
  await api.login(DEFAULT_E2E_EMAIL, DEFAULT_E2E_NAME);
  await api.ensureWorkspace("E2E Workspace", DEFAULT_E2E_WORKSPACE);
  return api;
}

export async function openWorkspaceMenu(page: Page) {
  await page.locator('[data-sidebar="menu-button"]').first().click();
  await page.getByRole("menu").waitFor({ state: "visible" });
}
