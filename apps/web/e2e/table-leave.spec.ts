import { expect, test } from "@playwright/test";

test("leaving a table clears recovery in every browser tab", async ({ context, page }) => {
  await page.goto("/");
  await page.getByLabel("Nama di meja").fill("Leave Sync Guest");
  await page.getByRole("button", { name: "Masuk sebagai tamu" }).click();
  await expect(page).toHaveURL(/\/lobby/);
  await page.getByRole("button", { name: "Buat meja" }).click();
  await expect(page).toHaveURL(/\/table\//);
  await expect(page.locator(".connection-status")).toContainText("Terhubung");

  const secondTab = await context.newPage();
  await secondTab.goto("/");
  await expect(secondTab).toHaveURL(/\/table\//);
  await expect(secondTab.locator(".connection-status")).toContainText("Terhubung");

  await page.getByRole("button", { name: "Keluar", exact: true }).click();
  await expect(page).toHaveURL(/\/lobby/);
  await expect(secondTab).toHaveURL(/\/lobby/);

  await secondTab.reload();
  await expect(secondTab).toHaveURL(/\/lobby/);
  await expect(secondTab.evaluate(() => localStorage.getItem("bridgeyok.table.v1"))).resolves.toBeNull();
});
