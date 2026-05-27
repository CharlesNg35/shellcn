import { test, expect } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  await request.post("/api/__test/reset");
});

test("add a connection from a protocol and open it", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Add connection" }).click();

  // Pick a protocol → its config schema renders.
  await page.getByRole("radio", { name: /^SSH/ }).click();

  await page.locator("#conn-name").fill("e2e-box");
  await page.getByPlaceholder("10.0.0.1").fill("10.0.0.9");
  await page.getByRole("button", { name: /Create connection/ }).click();

  // It is routed into the workspace and appears in the sidebar.
  await expect(page).toHaveURL(/\/c\/conn-/);
  await expect(page.locator("aside")).toContainText("e2e-box");
});

test("edit a connection and persist the change", async ({ page }) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: /prod-web-01/ })
    .first()
    .click();
  await expect(page).toHaveURL(/\/c\/ssh-prod-web/);

  await page.getByRole("button", { name: "Edit connection" }).click();
  await page.locator("#conn-name").fill("prod-web-renamed");
  await page.getByPlaceholder("10.0.0.1").fill("10.0.0.1");
  await page.getByRole("button", { name: "Save changes" }).click();

  await expect(page.locator("aside")).toContainText("prod-web-renamed");
});

test("delete a connection after confirmation", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Add connection" }).click();
  await page.getByRole("radio", { name: /^SSH/ }).click();
  await page.locator("#conn-name").fill("e2e-delete-me");
  await page.getByPlaceholder("10.0.0.1").fill("10.0.0.8");
  await page.getByRole("button", { name: /Create connection/ }).click();
  await expect(page.locator("aside")).toContainText("e2e-delete-me");

  await page.getByRole("button", { name: "Delete connection" }).click();
  await page.getByRole("button", { name: "Delete", exact: true }).click();

  await expect(page).not.toHaveURL(/\/c\/conn-/);
  await expect(page.locator("aside")).not.toContainText("e2e-delete-me");
});

test("create a credential from the credentials view", async ({ page }) => {
  await page.goto("/credentials");

  await page.getByRole("button", { name: /New credential/ }).click();
  await page.locator("#cred-name").fill("e2e-cred");
  await page.locator('input[type="password"]').first().fill("s3cret-value");
  await page.getByRole("button", { name: "Create credential" }).click();

  await expect(
    page.getByRole("cell", { name: "e2e-cred", exact: true }),
  ).toBeVisible();
});

test("view the recordings index", async ({ page }) => {
  await page.goto("/recordings");

  await expect(
    page.getByRole("heading", { name: "All Recordings" }),
  ).toBeVisible();
  await expect(page.getByRole("cell", { name: "prod-web-01" })).toBeVisible();
  await expect(page.getByText("Terminal", { exact: true })).toBeVisible();
  await expect(page.getByText("Desktop", { exact: true })).toBeVisible();
  await expect(page.getByText("demo", { exact: true }).first()).toBeVisible();
});

test("create a credential and select it from a connection credential_ref", async ({
  page,
}) => {
  await page.goto("/credentials");
  await page.getByRole("button", { name: /New credential/ }).click();
  await page.getByRole("combobox", { name: "Database password" }).click();
  await page.getByRole("option", { name: "SSH private key" }).click();
  await page.locator("#cred-name").fill("e2e-selectable-cred");
  await page.locator("textarea").fill("s3cret-value");
  await page.getByRole("button", { name: "Create credential" }).click();
  await expect(
    page.getByRole("cell", { name: "e2e-selectable-cred", exact: true }),
  ).toBeVisible();

  await page.getByRole("button", { name: "Add connection" }).click();
  await page.getByRole("radio", { name: /^SSH/ }).click();
  await page.locator("#conn-name").fill("e2e-credential-conn");
  await page.getByPlaceholder("10.0.0.1").fill("10.0.0.7");
  await page.getByRole("combobox", { name: "Password" }).click();
  await page.getByText("Stored credential", { exact: true }).click();
  await page.getByText("Select a credential").click();
  await page.getByText("e2e-selectable-cred · SSH private key").click();
  await page.getByRole("button", { name: /Create connection/ }).click();

  await expect(page.locator("aside")).toContainText("e2e-credential-conn");
});

test("share and revoke a connection grant", async ({ page }) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: /prod-web-01/ })
    .first()
    .click();

  await page.getByRole("button", { name: "Share connection" }).click();
  await page.getByPlaceholder("Select a user").fill("bob");
  await page.getByText("Bob Reyes (bob)").click();
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(
    page.getByRole("dialog", { name: /Share/ }).getByText("bob"),
  ).toBeVisible();

  await page.getByRole("button", { name: "Revoke bob" }).click();
  await page.getByRole("button", { name: "Revoke", exact: true }).click();
  await expect(page.getByText("Not shared with anyone yet.")).toBeVisible();
});
