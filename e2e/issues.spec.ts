import { test, expect } from "@playwright/test";
import { loginAsDefault, createTestApi } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Issues", () => {
  let api: TestApiClient | undefined;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    await api.createIssue("E2E Baseline " + Date.now());
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api?.cleanup();
  });

  test("issues page loads with board view", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Issues", exact: true }),
    ).toBeVisible();

    // Board columns should be visible
    await expect(page.locator("text=Backlog")).toBeVisible();
    await expect(page.locator("text=Todo")).toBeVisible();
    await expect(page.locator("text=In Progress")).toBeVisible();
  });

  test("can switch between board and list view", async ({ page }) => {
    const viewButton = page.getByRole("button", { name: "Board view" });
    await expect(viewButton).toBeVisible();

    await viewButton.click();
    await page.getByRole("menuitem", { name: "List", exact: true }).click();
    await expect(page.getByRole("button", { name: "List view" })).toBeVisible();

    await page.getByRole("button", { name: "List view" }).click();
    await page.getByRole("menuitem", { name: "Board", exact: true }).click();
    await expect(page.locator("text=Backlog")).toBeVisible();
  });

  test("can create a new issue", async ({ page }) => {
    await page.getByRole("button", { name: "New Issue" }).click();

    const title = "E2E Created " + Date.now();
    await page.getByRole("textbox", { name: "Issue title" }).fill(title);
    await page.getByRole("button", { name: "Create Issue" }).click();

    // New issue should appear on the page
    await expect(page.locator(`text=${title}`).first()).toBeVisible({
      timeout: 10000,
    });
  });

  test("can navigate to issue detail page", async ({ page }) => {
    // Create a known issue via API so the test controls its own fixture
    const issue = await api!.createIssue("E2E Detail Test " + Date.now());

    await page.goto(`/issues/${issue.id}`, { waitUntil: "domcontentloaded" });

    // Should show Properties panel
    await expect(page.locator("text=Properties")).toBeVisible();
    // Should show breadcrumb link back to Issues
    await expect(
      page.locator("a", { hasText: "Issues" }).first(),
    ).toBeVisible();
  });

  test("can cancel issue creation", async ({ page }) => {
    await page.getByRole("button", { name: "New Issue" }).click();

    await expect(
      page.getByRole("textbox", { name: "Issue title" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Close" }).click();

    await expect(
      page.getByRole("textbox", { name: "Issue title" }),
    ).not.toBeVisible();
    await expect(page.getByRole("button", { name: "New Issue" })).toBeVisible();
  });
});
