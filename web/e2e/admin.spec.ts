import { test, expect } from "@playwright/test";

test("admin creates a user from the Users view", async ({ page }) => {
  await page.goto("/settings");
  await page.getByRole("link", { name: "Users & access" }).click();
  await expect(page).toHaveURL(/\/settings\/users/);

  await page.getByRole("button", { name: "New user" }).click();
  await page.locator("#user-username").fill("e2e-user");
  await page.locator('input[type="password"]').fill("s3cret-pw");
  await page.getByRole("button", { name: "Create user" }).click();

  await expect(
    page.getByRole("cell", { name: "e2e-user", exact: true }),
  ).toBeVisible();
});

test("admin invites a user and the invitee accepts via the link", async ({
  page,
}) => {
  await page.goto("/settings/users");
  await page.getByRole("tab", { name: "Invitations" }).click();

  await page.getByRole("button", { name: "Invite user" }).click();
  await page.locator("#invite-email").fill("invitee@example.com");
  await page.getByRole("button", { name: "Create invitation" }).click();

  // The copyable link is revealed; derive the accept route from it.
  const link = await page.getByText(/\/invite\//).innerText();
  const token = link.split("/invite/")[1];
  expect(token).toBeTruthy();

  await page.goto(`/invite/${token}`);
  await expect(page.locator("body")).toContainText("invitee@example.com");
  await page.locator("#invite-username").fill("invited-user");
  await page.locator('input[type="password"]').fill("s3cret-pw");
  await page.getByRole("button", { name: "Create account" }).click();

  // Account created → the invitee leaves the accept page (in mock mode the
  // always-on session then lands on home rather than the login screen).
  await expect(page).not.toHaveURL(/\/invite\//);
});
