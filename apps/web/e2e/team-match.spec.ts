import { expect, test, type BrowserContext, type Page } from "@playwright/test";

test("eight players create, consent, play isolated rooms and read final IMP on responsive UI", async ({ browser }, testInfo) => {
  test.setTimeout(240000);
  const contexts: BrowserContext[] = [];
  const pages: Page[] = [];
  const prefix = `tm${Date.now().toString(36)}`;
  const sizes = [[320, 700], [390, 844], [768, 1024], [1024, 768], [1440, 900], [1920, 1080]];
  try {
    for (let _index = 0; _index < 8; _index++) {
      const context = await browser.newContext();
      contexts.push(context);
      const page = await context.newPage();
      pages.push(page);
      const response = await page.request.post("http://localhost:3100/api/account/signup", { headers: { Origin: "http://localhost:3100" }, data: { username: `${prefix}_${_index}`, displayName: `Match Player ${_index}`, password: "bridge match browser password", avatar: "spade" } });
      expect(response.ok(), await response.text()).toBe(true);
    }
    const owner = pages[0]!;
    await owner.goto("http://localhost:3100/play");
    await owner.getByRole("link", { name: /Team Match/ }).click();
    await owner.getByRole("button", { name: "Susun pemain" }).click();
    await owner.getByLabel("Jumlah board").fill("1");
    for (const [width, height] of sizes) {
      await owner.setViewportSize({ width: width!, height: height! });
      expect(await owner.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await owner.screenshot({ path: testInfo.outputPath(`setup-${width}.png`), fullPage: true });
    }
    await owner.setViewportSize({ width: 390, height: 844 });
    for (let _index = 1; _index < 8; _index++) {
      await owner.getByRole("searchbox", { name: "Cari pemain" }).fill(`${prefix}_${_index}`);
      await owner.getByRole("button", { name: `Pilih Match Player ${_index}`, exact: true }).click();
    }
    await owner.getByRole("button", { name: "Buat dan undang 8 pemain" }).click();
    await expect(owner).toHaveURL(/\/match\/[a-f0-9-]+$/);
    const matchURL = owner.url();
    await expect(owner.getByRole("button", { name: "Mulai match" })).toBeDisabled();
    for (let _index = 0; _index < pages.length; _index++) {
      const page = pages[_index]!;
      if (_index > 0) {
        await page.goto("http://localhost:3100/match");
        await page.getByRole("link", { name: /Tim [AB] · (OPEN|CLOSED)/ }).click();
      }
      await page.getByRole("button", { name: "Saya siap", exact: true }).click();
      await expect(page.getByRole("button", { name: "Batalkan siap" })).toBeEnabled();
    }
    await expect(owner.getByRole("button", { name: "Mulai match" })).toBeEnabled();
    await owner.getByRole("button", { name: "Mulai match" }).click();
    await expect(owner.getByRole("heading", { name: "Sedang dimainkan" })).toBeVisible();
    for (const page of pages) {
      await expect(page.getByRole("heading", { name: "Sedang dimainkan" })).toBeVisible();
      await page.getByRole("link", { name: /Masuk room/ }).click();
      await expect(page.locator(".own-hand .hand-card-slot")).toHaveCount(13);
      await expect(page.getByRole("button", { name: /^DD / })).toHaveCount(0);
    }
    for (const [width, height] of sizes) {
      await owner.setViewportSize({ width: width!, height: height! });
      expect(await owner.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await owner.screenshot({ path: testInfo.outputPath(`table-${width}.png`) });
    }
    await owner.setViewportSize({ width: 390, height: 844 });
    for (let _roomIndex = 0; _roomIndex < 2; _roomIndex++) {
      for (let _seatIndex = 0; _seatIndex < 4; _seatIndex++) {
        const player = pages[_roomIndex * 4 + _seatIndex]!;
        await player.getByRole("button", { name: /^Pass/ }).click();
      }
      const roomOwner = pages[_roomIndex * 4]!;
      await roomOwner.getByLabel("Buka menu meja").click();
      await roomOwner.getByRole("button", { name: "Selesaikan room" }).click();
    }
    await owner.getByRole("button", { name: "Kembali ke match" }).click();
    await expect(owner).toHaveURL(matchURL);
    await expect(owner.getByRole("heading", { name: "Hasil akhir · IMP" })).toBeVisible();
    await expect(owner.getByText("Seri.", { exact: true })).toBeVisible();
    await expect(owner.locator(".match-scores tbody tr")).toHaveCount(1);
    for (const [width, height] of sizes) {
      await owner.setViewportSize({ width: width!, height: height! });
      expect(await owner.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await owner.screenshot({ path: testInfo.outputPath(`result-${width}.png`), fullPage: true });
    }
    expect((await owner.getByRole("link", { name: /Masuk room/ }).boundingBox())!.height).toBeGreaterThanOrEqual(44);
    await owner.reload();
    await expect(owner.getByText("Seri.", { exact: true })).toBeVisible();
  } finally {
    for (const context of contexts) await context.close();
  }
});
