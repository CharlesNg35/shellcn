import { test, expect } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  await request.post("/api/__test/reset");
});

test("audit filters sit above the table and remain usable on mobile", async ({
  page,
}) => {
  await page.goto("/settings/users/u-demo");
  await page.getByRole("tab", { name: "Audit" }).click();

  const search = page.getByLabel("Search audit events");
  const table = page.getByRole("table");
  const filterBar = page.getByTestId("audit-filter-bar");
  await expect(search).toBeVisible();
  await expect(page.getByText("Result").first()).toBeVisible();
  await expect(page.getByText("Risk").first()).toBeVisible();
  await expect(page.getByRole("button", { name: /More/ })).toBeVisible();
  await expect(table).toBeVisible();

  const searchBox = await search.boundingBox();
  const filterBarBox = await filterBar.boundingBox();
  const tableBox = await table.boundingBox();
  expect(searchBox?.y).toBeLessThan(tableBox?.y ?? 0);
  expect(filterBarBox?.height ?? 999).toBeLessThan(56);

  const filtered = page.waitForResponse((response) => {
    const url = response.url();
    return (
      response.request().method() === "GET" &&
      url.includes("/api/admin/users/u-demo/audit") &&
      url.includes("event=login")
    );
  });
  await search.fill("login");
  await filtered;

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(search).toBeVisible();
  await expect(page.getByRole("button", { name: /More/ })).toBeVisible();
  await expect(table).toBeVisible();

  const scrollWidth = await page.evaluate(
    () => document.documentElement.scrollWidth,
  );
  const viewportWidth = await page.evaluate(() => window.innerWidth);
  expect(scrollWidth).toBeLessThanOrEqual(viewportWidth);
});
