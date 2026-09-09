import { expect, test, type Page } from "@playwright/test";

async function signup(page: Page, username: string, name: string) {
  await page.goto("/signup");
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Nama di meja").fill(name);
  await page.getByLabel("Kata sandi", { exact: true }).fill("bridge test password");
  await page.getByRole("button", { name: "Sign Up", exact: true }).click();
  await expect(page).toHaveURL(/\/play$/);
}

async function noOverflow(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}

test("account guards, profile, navigation and responsive critical path", async ({ page }, testInfo) => {
  test.setTimeout(180000);
  await page.goto("/friends");
  await expect(page).toHaveURL(/\/login\?next=%2Ffriends/);
  const sizes = [[320, 700], [390, 844], [768, 1024], [1024, 768], [1440, 900], [1920, 1080]];
  for (const [width, height] of sizes) {
    await page.setViewportSize({ width: width!, height: height! });
    await page.goto("/");
    await noOverflow(page);
    const cta = await page.getByRole("link", { name: /Get Started/ }).boundingBox();
    expect(cta!.y + cta!.height).toBeLessThan(height!);
    await page.screenshot({ path: testInfo.outputPath(`landing-${width}.png`) });
  }
  const username = `a_${Date.now()}`;
  await signup(page, username, "Nama Panjang Dua Puluh");
  for (const route of ["/", "/login", "/signup"]) {
    await page.goto(route);
    await expect(page).toHaveURL(/\/play$/);
  }
  for (const [width, height] of sizes) {
    await page.setViewportSize({ width: width!, height: height! });
    await noOverflow(page);
    const navigation = page.getByRole("navigation", { name: "Navigasi utama" });
    await expect(navigation.getByRole("link", { name: "Play", exact: true })).toHaveAttribute("aria-current", "page");
    if (width! < 768) {
      const play = await navigation.getByRole("link", { name: "Play", exact: true }).boundingBox();
      expect(Math.abs(play!.x + play!.width / 2 - width! / 2)).toBeLessThan(2);
      expect(play!.height).toBeGreaterThanOrEqual(44);
    }
    await page.getByRole("button", { name: /Team Match/ }).click();
    await expect(page.getByRole("status").filter({ hasText: "Coming soon" })).toBeVisible();
    await page.getByRole("button", { name: "Tutup pesan" }).click();
    await navigation.getByRole("button", { name: "History" }).click();
    await expect(page.getByRole("status").filter({ hasText: "Coming soon" })).toBeVisible();
    await page.getByRole("button", { name: "Tutup pesan" }).click();
    await page.screenshot({ path: testInfo.outputPath(`play-${width}.png`) });
    await navigation.getByRole("link", { name: "Settings", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Profile", exact: true })).toBeVisible();
    await noOverflow(page);
    await page.screenshot({ path: testInfo.outputPath(`profile-${width}.png`) });
    await navigation.getByRole("link", { name: "Play", exact: true }).click();
  }
  await page.getByRole("button", { name: /Team Match/ }).click();
  await page.getByRole("button", { name: /VS Robot/ }).click();
  await expect(page.getByRole("status").filter({ hasText: "Coming soon" })).toHaveCount(1);
  await page.getByRole("button", { name: "Tutup pesan" }).click();
  for (const route of ["/team-match", "/robot", "/teacher", "/history", "/deals"]) {
    const response = await page.goto(route);
    expect(response?.status()).toBe(404);
  }
  await page.goto("/settings");
  await page.getByLabel("Nama di meja").fill("Updated Player");
  await page.getByRole("radio", { name: "fox", exact: true }).check();
  await page.getByRole("button", { name: "Simpan profile" }).click();
  await expect(page.getByRole("status").filter({ hasText: "Profile tersimpan" })).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Nama di meja")).toHaveValue("Updated Player");
  await expect(page.getByRole("radio", { name: "fox", exact: true })).toBeChecked();
  await page.getByRole("button", { name: "Logout", exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/table/00000000-0000-4000-8000-000000000000");
  await expect(page).toHaveURL(/\/login\?next=%2Ftable/);
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Kata sandi", { exact: true }).fill("bridge test password");
  await page.getByRole("button", { name: "Login", exact: true }).click();
  await expect(page).toHaveURL(/\/(table|lobby)/);
});

test("mutual friends, participant follow and online invite preserve table authority", async ({ browser }, testInfo) => {
  test.setTimeout(180000);
  const first = await browser.newContext();
  const second = await browser.newContext();
  const alice = await first.newPage();
  const bob = await second.newPage();
  const suffix = Date.now();
  const aliceUser = `alice_${suffix}`;
  const bobUser = `bob_${suffix}`;
  await signup(alice, aliceUser, "Alice");
  await signup(bob, bobUser, "Bob");
  await alice.goto("/friends");
  await alice.getByLabel("Friends saja").uncheck();
  await alice.getByRole("searchbox").fill(bobUser);
  await alice.getByRole("button", { name: "Follow Bob", exact: true }).click();
  await expect(alice.getByRole("button", { name: "Unfollow Bob" })).toHaveText("Following · Unfollow");
  await bob.goto("/friends");
  await bob.getByLabel("Friends saja").uncheck();
  await bob.getByRole("searchbox").fill(aliceUser);
  await bob.getByRole("button", { name: "Follow Alice", exact: true }).click();
  await expect(bob.getByRole("button", { name: "Unfollow Alice" })).toHaveText("Friends · Unfollow");
  await alice.reload();
  await expect(alice.getByRole("button", { name: "Unfollow Bob" })).toHaveText("Friends · Unfollow");
  await alice.goto("/lobby");
  await alice.getByRole("button", { name: "Buat meja" }).click();
  await expect(alice).toHaveURL(/\/table\//);
  await expect(alice.locator(".connection-status")).toContainText("Terhubung");
  const tableURL = alice.url();
  await alice.getByRole("button", { name: "Invite player", exact: true }).click();
  await alice.getByRole("searchbox").fill(bobUser);
  await expect(alice.getByRole("button", { name: "Invite", exact: true })).toBeEnabled();
  await bob.goto("/settings");
  await bob.getByRole("button", { name: "Logout", exact: true }).click();
  await expect(bob).toHaveURL(/\/login$/);
  await expect(alice.getByRole("button", { name: "Invite", exact: true })).toBeDisabled({ timeout: 20000 });
  await expect(alice.getByText("Pemain offline", { exact: true })).toBeVisible();
  await bob.getByLabel("Username", { exact: true }).fill(bobUser);
  await bob.getByLabel("Kata sandi", { exact: true }).fill("bridge test password");
  await bob.getByRole("button", { name: "Login", exact: true }).click();
  await expect(bob).toHaveURL(/\/play$/);
  await expect(alice.getByRole("button", { name: "Invite", exact: true })).toBeEnabled({ timeout: 20000 });
  for (const [width, height] of [[320, 700], [390, 844], [768, 1024], [1024, 768], [1440, 900], [1920, 1080]]) {
    await alice.setViewportSize({ width: width!, height: height! });
    await noOverflow(alice);
    const dialog = await alice.getByRole("dialog").boundingBox();
    expect(dialog!.width).toBeLessThan(width!);
    expect(dialog!.height).toBeLessThan(height!);
    await alice.screenshot({ path: testInfo.outputPath(`invite-${width}.png`) });
  }
  await alice.getByRole("button", { name: "Invite", exact: true }).click();
  await expect(alice.getByRole("status").filter({ hasText: "Undangan terkirim" })).toBeVisible();
  await expect(bob.getByRole("link", { name: "Lihat meja" })).toBeVisible({ timeout: 20000 });
  await alice.getByRole("button", { name: "Tutup invite" }).click();
  await expect(alice.getByRole("button", { name: "Invite player", exact: true })).toBeFocused();
  await expect(alice.getByText("1/4 pemain sudah masuk.", { exact: false })).toBeVisible();
  await bob.getByRole("link", { name: "Lihat meja" }).click();
  await bob.getByRole("button", { name: "Masuk", exact: true }).click();
  await expect(bob).toHaveURL(tableURL);
  await bob.getByRole("button", { name: "Buka menu kursi kosong E" }).click();
  await bob.getByRole("button", { name: "Duduk", exact: true }).click();
  await alice.setViewportSize({ width: 390, height: 844 });
  await expect(alice.getByRole("button", { name: "Buka menu Bob, kursi E" })).toBeVisible();
  await alice.getByRole("button", { name: "Buka menu Bob, kursi E" }).click();
  await expect(alice.getByRole("button", { name: "Unfollow Bob" })).toBeVisible({ timeout: 20000 });
  await alice.getByRole("button", { name: "Unfollow Bob" }).click();
  await expect(alice.getByRole("button", { name: "Follow Bob", exact: true })).toBeVisible();
  await alice.keyboard.press("Escape");
  await noOverflow(alice);
  await first.close();
  await second.close();
});

test("mobile touch navigation, search states and keyboard recovery", async ({ browser }, testInfo) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
  const page = await context.newPage();
  const username = `touch_${Date.now()}`;
  await signup(page, username, "Touch Player");
  const navigation = page.getByRole("navigation", { name: "Navigasi utama" });
  await navigation.getByRole("link", { name: "Friends", exact: true }).tap();
  await expect(page.getByRole("status").filter({ hasText: "Belum ada Friends" })).toBeVisible();
  await page.getByLabel("Friends saja").uncheck();
  await page.getByRole("searchbox").fill(username);
  await expect(page.getByText("Kamu", { exact: true })).toBeVisible();
  await page.getByRole("searchbox").fill("zz_no_such_account_zz");
  await expect(page.getByRole("status").filter({ hasText: "Tidak ada pemain" })).toBeVisible();
  await page.route("**/api/account/users?*", route => route.fulfill({ status: 503, contentType: "application/json", body: '{"code":"SERVICE_UNAVAILABLE"}' }));
  await page.getByRole("searchbox").fill("retry");
  await expect(page.getByRole("region", { name: "Friends dan pencarian" }).getByRole("alert")).toContainText("Layanan belum terhubung");
  await page.unroute("**/api/account/users?*");
  await page.getByRole("button", { name: "Coba lagi", exact: true }).tap();
  await expect(page.getByRole("region", { name: "Friends dan pencarian" }).getByRole("alert")).toHaveCount(0);
  await page.getByRole("searchbox").fill(username);
  await expect(page.getByText("Kamu", { exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("friends-touch-390.png") });
  await navigation.getByRole("link", { name: "Settings", exact: true }).tap();
  await page.getByRole("radio", { name: "heart", exact: true }).check();
  await page.getByRole("button", { name: "Simpan profile" }).tap();
  await expect(page.getByRole("status").filter({ hasText: "Profile tersimpan" })).toBeVisible();
  await navigation.getByRole("link", { name: "Play", exact: true }).tap();
  await page.getByRole("button", { name: /Teacher Table/ }).tap();
  await expect(page.getByRole("status").filter({ hasText: "Coming soon" })).toBeVisible();
  await page.getByRole("button", { name: "Tutup pesan" }).tap();
  await navigation.getByRole("link", { name: "Friends", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/friends$/);
  await noOverflow(page);
  await context.close();
});
