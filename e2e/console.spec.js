const { randomUUID } = require("node:crypto");
const { expect, test } = require("@playwright/test");

async function signIn(page, email) {
  await page.goto("/auth/sign_in");
  await page.getByLabel("Email address").fill(email);
  await page.getByLabel("Password", { exact: true }).fill("password123");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Dashboard", exact: true })).toBeVisible();
}

test("an administrator updates a product and its SKU", async ({ page }) => {
  await signIn(page, "admin@example.com");

  const run = randomUUID().slice(0, 8);
  const name = `E2E product ${run}`;
  await page.getByRole("link", { name: /Create product/ }).click();
  await page.getByLabel(/Product name/).fill(name);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page).toHaveURL(/\/console\/products\/\d+$/);

  await page.getByRole("link", { name: "Edit product" }).click();
  const updatedName = `${name} updated`;
  await page.getByLabel(/Product name/).fill(updatedName);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText(updatedName, { exact: true })).toBeVisible();

  const code = `E2E-${run.toUpperCase()}`;
  await page.getByRole("link", { name: /Add SKU/ }).click();
  await page.getByLabel(/SKU code/).fill(code);
  await page.getByLabel("Price (JPY)").fill("1250");
  await page.getByLabel("Stock").fill("3");
  await page.getByRole("button", { name: "Save" }).click();

  const sku = page.getByRole("row").filter({ has: page.getByRole("cell", { name: code, exact: true }) });
  await sku.getByRole("link", { name: "Edit", exact: true }).click();
  await page.getByLabel("Price (JPY)").fill("1800");
  await page.getByLabel("Stock").fill("7");
  await page.getByRole("button", { name: "Save" }).click();

  await page.reload();
  await expect(page.getByText(updatedName, { exact: true })).toBeVisible();
  await expect(sku.getByRole("cell", { name: "1800", exact: true })).toBeVisible();
  await expect(sku.getByRole("cell", { name: "7", exact: true })).toBeVisible();
});

test("an operator can list administrators and is denied the create page", async ({ page }) => {
  await signIn(page, "operator01@example.com");

  const list = await page.goto("/console/admins");
  expect(list.status()).toBe(200);
  await expect(page.getByRole("heading", { name: "Administrators", exact: true })).toBeVisible();

  const forbidden = await page.goto("/console/admins/new");
  expect(forbidden.status()).toBe(403);
  await expect(page.getByText("This action requires a super administrator account.")).toBeVisible();
});
