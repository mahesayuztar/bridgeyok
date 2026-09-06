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

test("active navbar stays usable across viewport sizes and confirms leaving", async ({ page }, testInfo) => {
  await page.goto("/");
  await page.getByLabel("Nama di meja").fill("Navbar Guest");
  await page.getByRole("button", { name: "Masuk sebagai tamu" }).click();
  await expect(page).toHaveURL(/\/lobby/);
  await page.getByRole("button", { name: "Buat meja" }).click();
  await expect(page.locator(".connection-status")).toContainText("Terhubung");
  await page.getByRole("button", { name: "Buka menu kursi kosong N" }).click();
  await page.getByRole("button", { name: "Duduk", exact: true }).click();
  for (const seat of ["E", "S", "W"]) {
    await page.getByRole("button", { name: `Buka menu kursi kosong ${seat}` }).click();
    await page.getByRole("button", { name: "Tambah bot" }).click();
    await expect(page.getByRole("button", { name: `Buka menu Bot, kursi ${seat}, bot` })).toBeVisible();
  }
  await page.getByRole("button", { name: "Buka menu Navbar Guest, kursi N" }).click();
  await page.getByRole("button", { name: "Saya siap" }).click();
  await page.getByRole("button", { name: "Mulai board" }).click();
  await expect(page.locator(".auction-workspace")).toBeVisible();

  for (const viewport of [
    { width: 320, height: 700 },
    { width: 360, height: 740 },
    { width: 390, height: 844 },
    { width: 400, height: 800 },
    { width: 768, height: 1024 },
    { width: 1440, height: 900 },
  ]) {
    await page.setViewportSize(viewport);
    for (const selector of [".table-leave-button", ".score-summary", ".board-marker", ".present-contract", ".consensus-navigation", ".status-actions"]) {
      const element = page.locator(selector);
      await expect(element).toBeVisible();
      const box = (await element.boundingBox())!;
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(viewport.width);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByRole("button", { name: "Buka skor meja" }).click();
    await expect(page.getByRole("dialog", { name: "History" })).toBeVisible();
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Buka riwayat auction" }).click();
    await expect(page.getByRole("dialog", { name: "Auction", exact: true })).toBeVisible();
    await page.keyboard.press("Escape");
    await page.getByRole("button", { name: "Keluar dari meja", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "Keluar dari meja?" });
    await expect(dialog.getByRole("button", { name: "Batal" })).toBeFocused();
    await page.keyboard.press("p");
    await expect(page.locator(".auction-workspace tbody")).toHaveText("");
    await page.screenshot({ path: testInfo.outputPath(`leave-confirmation-${viewport.width}.png`) });
    await dialog.getByRole("button", { name: "Batal" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByRole("button", { name: "Keluar dari meja", exact: true })).toBeFocused();
    await page.screenshot({ path: testInfo.outputPath(`navbar-${viewport.width}.png`) });
  }
  await page.getByRole("button", { name: "Keluar dari meja", exact: true }).click();
  await page.getByRole("dialog", { name: "Keluar dari meja?" }).getByRole("button", { name: "Keluar", exact: true }).click();
  await expect(page).toHaveURL(/\/lobby/);
  await expect(page.evaluate(() => localStorage.getItem("bridgeyok.table.v1"))).resolves.toBeNull();
});
