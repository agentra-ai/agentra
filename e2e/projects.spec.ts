import { test, expect } from "@playwright/test";
import { createTestApi, loginAsDefault } from "./helpers";
import type { TestApiClient } from "./fixtures";

test.describe("Projects", () => {
  let api: TestApiClient | undefined;
  let issueTitle: string;

  test.beforeEach(async ({ page }) => {
    api = await createTestApi();
    issueTitle = `E2E Project Issue ${Date.now()}`;
    await api.createIssue(issueTitle);
    await loginAsDefault(page);
  });

  test.afterEach(async () => {
    await api?.cleanup();
  });

  test("creates a project and assigns an issue", async ({ page }) => {
    await page.getByRole("link", { name: "Projects", exact: true }).click();
    await page.waitForURL("**/projects");

    await expect(
      page.getByRole("heading", { name: "Projects", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Unassigned Issues", exact: true }),
    ).toBeVisible();
    await expect(page.getByRole("link", { name: new RegExp(issueTitle) })).toBeVisible();

    const projectTitle = `E2E Project ${Date.now()}`;
    const projectSlug = `e2e-project-${Date.now()}`;
    await page.getByRole("button", { name: "New Project" }).click();
    await page.getByLabel("Project Title").fill(projectTitle);
    await page.getByLabel("Project Slug").fill(projectSlug);
    await page.getByRole("button", { name: "Create Project" }).click();

    await expect(
      page.getByRole("heading", { name: projectTitle, exact: true }),
    ).toBeVisible();

    const project = (await api!.listProjects()).find(
      (item) => item.slug === projectSlug,
    );
    expect(project).toBeTruthy();
    api!.trackProject(project!.id);

    await page.getByRole("button", { name: "Add Issue" }).click();
    await page.getByRole("button", { name: new RegExp(issueTitle) }).click();

    await expect(
      page.getByRole("link", { name: new RegExp(issueTitle) }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Remove from Project" }),
    ).toBeVisible();
  });
});
