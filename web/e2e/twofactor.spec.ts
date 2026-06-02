import { test, expect } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  await request.post("/api/__test/reset");
});

test("enable two-factor authentication from the profile", async ({ page }) => {
  await page.goto("/settings/profile");

  await expect(
    page.getByRole("heading", { name: "Two-factor authentication" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Enable 2FA" }).click();

  const dialog = page.getByRole("dialog", { name: /Enable two-factor/ });
  await expect(
    dialog.getByRole("img", { name: "Two-factor QR code" }),
  ).toBeVisible();

  await dialog.getByLabel("Verification code").fill("123456");
  await dialog.getByRole("button", { name: "Enable 2FA" }).click();

  // Recovery codes are revealed; acknowledging unlocks Done.
  await expect(dialog.getByText("aaaa-bbbb")).toBeVisible();
  await dialog.getByText("I've saved my recovery codes.").click();
  await dialog.getByRole("button", { name: "Done" }).click();

  await expect(page.getByRole("button", { name: "Disable 2FA" })).toBeVisible();
});

test("an admin resets a locked-out user's two-factor", async ({ page }) => {
  await page.goto("/settings/users/u-bob");

  await expect(page.getByText("Two-factor", { exact: true })).toBeVisible();
  await expect(page.getByText("Enabled")).toBeVisible();

  await page.getByRole("button", { name: "Reset two-factor" }).click();
  await page.getByRole("button", { name: "Reset", exact: true }).click();

  await expect(page.getByText("Disabled")).toBeVisible();
});

test("the secure-account reminder can be dismissed", async ({ page }) => {
  await page.goto("/secure-account");

  await expect(
    page.getByRole("heading", { name: "Secure your account" }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Remind me later" }).click();
  await expect(page).not.toHaveURL(/secure-account/);
});
