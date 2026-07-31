import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Comments", () => {
  let api: TestApiClient | undefined;
  let issueId: string;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    const issue = await api.createIssue("E2E Comment Test " + Date.now());
    issueId = issue.id as string;
    await loginAsDefault(page);
    await page.goto(`/issues/${issueId}`, { waitUntil: "domcontentloaded" });
    await expect(page.getByText("Properties", { exact: true })).toBeVisible();
  });

  test.afterEach(async () => {
    await api?.cleanup();
  });

  test("can add a comment on an issue", async ({ page }) => {
    const commentText = "E2E comment " + Date.now();
    const commentInput = page.getByRole("textbox", {
      name: "Leave a comment...",
    });
    await commentInput.fill(commentText);

    await page.getByRole("button", { name: "Submit" }).click();

    // Comment should appear in the activity section
    await expect(page.locator(`text=${commentText}`)).toBeVisible({
      timeout: 5000,
    });
  });

  test("comment submit button is disabled when empty", async ({ page }) => {
    const submitBtn = page.getByRole("button", { name: "Submit" });
    await expect(submitBtn).toBeDisabled();
  });
});
