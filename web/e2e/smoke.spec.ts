import { test, expect } from "@playwright/test";

test("renders the app shell", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#app")).toContainText("ShellCN");
});
