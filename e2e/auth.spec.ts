import { test, expect } from "@playwright/test";
import { loginAsDefault, openWorkspaceMenu } from "./helpers";

test.describe("Authentication", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    await expect(page.getByText("Agentra", { exact: true })).toBeVisible();
    await expect(
      page.getByText("Turn coding agents into real teammates"),
    ).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Email" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign In" })).toBeVisible();
  });

  test("login and redirect to /issues", async ({ page }) => {
    await loginAsDefault(page);

    await expect(page).toHaveURL(/\/issues/);
    await expect(
      page.getByRole("heading", { name: "Issues", exact: true }),
    ).toBeVisible();
  });

  test("unauthenticated user is redirected to the landing page", async ({
    page,
  }) => {
    await page.goto("/login");
    await page.evaluate(() => {
      localStorage.removeItem("agentra_token");
      localStorage.removeItem("agentra_workspace_id");
    });

    await page.goto("/issues");
    await page.waitForURL((url) => url.pathname === "/", { timeout: 10000 });
  });

  test("logout redirects to the landing page", async ({ page }) => {
    await loginAsDefault(page);

    // Open the workspace dropdown menu
    await openWorkspaceMenu(page);

    await page.getByRole("menuitem", { name: "Logout" }).click();

    await page.waitForURL((url) => url.pathname === "/", { timeout: 10000 });
  });
});
