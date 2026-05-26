import { test, expect } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  await request.post("/api/__test/reset");
});

test("renders the app shell", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#app")).toContainText("ShellCN");
});

test("SSH (tabs): terminal stream, home files, snippets", async ({ page }) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: /prod-web-01/ })
    .first()
    .click();
  await expect(page).toHaveURL(/\/c\/ssh-prod-web/);
  await page
    .getByRole("main")
    .getByRole("button", { name: "Connect", exact: true })
    .click();

  await expect(page.locator("main")).toContainText(
    "Connected to mock shell. Type and press enter.",
  );

  // Files tab: list + type-based preview.
  await page.getByRole("tab", { name: "Files" }).click();
  await expect(page.locator("main")).toContainText("app.json");
  await expect(page.locator("main")).toContainText("home");
  await page.getByRole("button", { name: /app\.json/ }).click();
  await expect(page.locator("textarea")).toHaveValue(/"name": "app"/);

  await page.getByRole("tab", { name: "Snippets" }).click();
  await expect(page.locator("main")).toContainText("disk usage");
  await expect(page.getByRole("button", { name: "New snippet" })).toBeVisible();
  await page.getByText("disk usage").click();
  await expect(
    page.getByRole("button", { name: "Run", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Run", exact: true }).click();
  await page.getByRole("button", { name: "Confirm", exact: true }).click();
  await expect(
    page.getByRole("application", { name: "Terminal session" }),
  ).toBeVisible();
});

test("Docker (sidebar tree): list table + resource detail", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "docker-local" }).click();
  await page
    .getByRole("main")
    .getByRole("button", { name: "Connect", exact: true })
    .click();

  // Select the Containers group → resource list table.
  await page.getByRole("treeitem", { name: /Containers/ }).click();
  await expect(page.locator("main")).toContainText("nginx-1");

  // Click a row → detail view with its tabs.
  await page.getByRole("cell", { name: "nginx-1", exact: true }).click();
  await expect(page.locator("main")).toContainText("Logs");
  await expect(page.locator("main")).toContainText("Inspect");
});

test("Docker agent connection shows the enroll panel", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: /edge-host/ }).click();
  await page.getByRole("button", { name: "Set up agent" }).click();
  await expect(page.locator("main")).toContainText("Connect the agent");
  await expect(page.locator("main")).toContainText("Generate install command");
});

test("Proxmox: deep tree to a VM detail with a console tab", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "pve-dc1" }).click();
  await page
    .getByRole("main")
    .getByRole("button", { name: "Connect", exact: true })
    .click();
  await page.getByRole("treeitem", { name: /Nodes/ }).click();
  await page.getByRole("treeitem", { name: /pve1$/ }).click();
  await page.getByRole("treeitem", { name: /pve1-vm-1/ }).click();
  await expect(page.locator("main")).toContainText("Console");
  await expect(page.locator("main")).toContainText("Snapshots");
});

test("PostgreSQL: schema tree to a table with a query editor", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "main-db" }).click();
  await page
    .getByRole("main")
    .getByRole("button", { name: "Connect", exact: true })
    .click();
  await page.getByRole("treeitem", { name: /Databases/ }).click();
  await page.getByRole("treeitem", { name: /^app$/ }).click();
  await page.getByRole("treeitem", { name: /Tables/ }).click();
  await page.getByRole("treeitem", { name: /^users$/ }).click();
  await expect(page.locator("main")).toContainText("Data");
  await expect(page.locator("main")).toContainText("Schema");
});
