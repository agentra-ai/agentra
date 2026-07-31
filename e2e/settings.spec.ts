import { test, expect } from "@playwright/test";
import { loginAsDefault } from "./helpers";

test.describe("Settings", () => {
  test("updating workspace name reflects in sidebar immediately", async ({
    page,
  }) => {
    await loginAsDefault(page);

    // Read the current workspace name from the sidebar
    const workspaceSwitcher = page.getByTestId("workspace-switcher");
    const originalName = await workspaceSwitcher.getAttribute("aria-label");
    if (!originalName) throw new Error("Workspace switcher has no accessible name");

    await page.getByRole("link", { name: "Settings", exact: true }).click();
    await page.waitForURL("**/settings");
    await page.getByRole("tab", { name: "General", exact: true }).click();

    const nameInput = page.getByRole("textbox", { name: "Workspace Name" });
    await nameInput.clear();
    const newName = "Renamed WS " + Date.now();
    await nameInput.fill(newName);

    // Save
    await page.getByRole("button", { name: "Save", exact: true }).click();

    await expect(page.getByText("Workspace settings saved").last()).toBeVisible({
      timeout: 5000,
    });

    // Sidebar should reflect the new name WITHOUT page refresh
    await expect(workspaceSwitcher).toHaveAccessibleName(newName);

    // Restore original name so other tests aren't affected
    await nameInput.clear();
    await nameInput.fill(originalName);
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(workspaceSwitcher).toHaveAccessibleName(originalName);
  });
});
