import { expect, test, type Page } from "@playwright/test";

async function signup(page: Page, username: string) {
  await page.goto("/signup");
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Nama di meja").fill(username);
  await page
    .getByLabel("Kata sandi", { exact: true })
    .fill("bridge chat password");
  await page.getByRole("button", { name: "Sign Up", exact: true }).click();
  await expect(page).toHaveURL(/\/play$/);
  return page.evaluate(
    async () =>
      (await (await fetch("/api/account")).json()).profile as { id: string },
  );
}

test("private chat optimistic realtime, offline retained history, emoji, toast and responsive input", async ({
  browser,
}) => {
  test.setTimeout(180000);
  const aliceContext = await browser.newContext();
  const bobContext = await browser.newContext();
  await aliceContext.routeWebSocket("ws://localhost:8180/v1/ws?*", (route) => {
    const server = route.connectToServer();
    server.onMessage((message) => {
      const envelope = JSON.parse(String(message)) as { name?: string };
      if (envelope.name?.startsWith("chat."))
        setTimeout(() => route.send(message), 500);
      else route.send(message);
    });
  });
  const alice = await aliceContext.newPage();
  let bob = await bobContext.newPage();
  const suffix = Date.now();
  const aliceName = `ca_${suffix}`;
  const bobName = `cb_${suffix}`;
  const first = await signup(alice, aliceName);
  const second = await signup(bob, bobName);
  await alice.evaluate(async (id) => {
    await fetch(`/api/account/users/${id}/follow`, { method: "PUT" });
  }, second.id);
  await bob.evaluate(async (id) => {
    await fetch(`/api/account/users/${id}/follow`, { method: "PUT" });
  }, first.id);
  await alice.goto("/friends");
  await alice
    .getByRole("button", { name: "Chat", exact: true })
    .first()
    .click();
  const panel = alice.getByRole("region", { name: `Chat ${bobName}` });
  await panel.getByLabel("Pesan", { exact: true }).fill("👩🏽‍💻 ♥️ hello");
  await panel.getByRole("button", { name: "Kirim", exact: true }).click();
  await expect(panel.getByText("👩🏽‍💻 ♥️ hello", { exact: true })).toBeVisible({
    timeout: 150,
  });
  await expect(panel.getByText(/Mengirim…/)).toBeVisible({ timeout: 150 });
  await expect(
    bob.getByRole("button", { name: "Open Chat", exact: true }),
  ).toBeVisible();
  await bob.getByRole("button", { name: "Open Chat", exact: true }).click();
  await expect(bob.getByText("👩🏽‍💻 ♥️ hello", { exact: true })).toHaveCount(1);
  await bob.getByLabel("Pesan", { exact: true }).fill("♠️ reply");
  await bob.getByRole("button", { name: "Kirim", exact: true }).click();
  await expect(panel.getByText("♠️ reply", { exact: true })).toHaveCount(1);
  await bob.close();
  await panel.getByLabel("Pesan", { exact: true }).fill("offline delivery");
  await panel.getByRole("button", { name: "Kirim", exact: true }).click();
  await expect(
    panel.getByText("offline delivery", { exact: true }),
  ).toBeVisible();
  bob = await bobContext.newPage();
  await bob.goto("/friends");
  await bob.getByRole("button", { name: "Chat", exact: true }).first().click();
  await expect(bob.getByText("offline delivery", { exact: true })).toHaveCount(
    1,
  );
  for (const [width, height] of [
    [320, 700],
    [375, 812],
    [390, 844],
    [768, 1024],
    [1024, 768],
    [1366, 768],
    [1440, 900],
    [1920, 1080],
  ]) {
    await alice.setViewportSize({ width: width!, height: height! });
    await panel.getByLabel("Pesan", { exact: true }).click();
    expect(
      await alice.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  }
  await aliceContext.close();
  await bobContext.close();
});

test("permanent table chat preserves gameplay geometry and pointer actions across viewport sizes", async ({
  browser,
}, testInfo) => {
  test.setTimeout(120000);
  const context = await browser.newContext({ hasTouch: true });
  const page = await context.newPage();
  const name = `ct_${Date.now()}`;
  await signup(page, name);
  await page.goto("/lobby");
  await page.getByRole("button", { name: "Buat meja", exact: true }).click();
  await expect(page).toHaveURL(/\/table\//);
  await expect(page.locator(".connection-status")).toContainText("Terhubung");
  await page.getByRole("button", { name: "Buka menu kursi kosong N" }).click();
  await page.getByRole("button", { name: "Duduk", exact: true }).click();
  for (const seat of ["E", "S", "W"]) {
    await page
      .getByRole("button", { name: `Buka menu kursi kosong ${seat}` })
      .click();
    await page.getByRole("button", { name: "Tambah bot", exact: true }).click();
  }
  await page
    .getByRole("button", { name: `Buka menu ${name}, kursi N` })
    .click();
  await page.getByRole("button", { name: "Saya siap", exact: true }).click();
  await page.getByRole("button", { name: "Mulai board", exact: true }).click();
  await expect(page.locator(".own-hand .physical-card")).toHaveCount(13);
  await expect(page.getByRole("button", { name: /^DD / })).toHaveCount(0);
  const tableMenuTrigger = page.getByRole("button", { name: "Buka menu meja", exact: true });
  await tableMenuTrigger.click();
  await expect(page.getByRole("button", { name: "Undo tidak tersedia", exact: true })).toBeVisible();
  await expect(page.getByText("Claim", { exact: true })).toBeVisible();
  await tableMenuTrigger.click();
  const panel = page.getByRole("region", { name: "Chat meja", exact: true });
  await expect(panel).toBeVisible();
  await panel.getByLabel("Pesan", { exact: true }).fill("Table ♠️");
  await panel.getByRole("button", { name: "Kirim", exact: true }).click();
  await expect(panel.getByText("Table ♠️", { exact: true })).toHaveCount(1);
  await page.setViewportSize({ width: 320, height: 700 });
  const levelGeometry = await page.locator(".bidding-box").evaluate((box) => {
    const levelButtons = [...box.querySelectorAll<HTMLElement>(".bid-levels button")];
    return {
      levelCount: levelButtons.length,
      minimumLevelWidth: Math.min(...levelButtons.map((button) => button.getBoundingClientRect().width)),
      minimumLevelHeight: Math.min(...levelButtons.map((button) => button.getBoundingClientRect().height)),
      overflowX: document.documentElement.scrollWidth > innerWidth,
      overflowY: document.documentElement.scrollHeight > innerHeight,
    };
  });
  expect(levelGeometry).toEqual({
    levelCount: expect.any(Number),
    minimumLevelWidth: expect.any(Number),
    minimumLevelHeight: expect.any(Number),
    overflowX: false,
    overflowY: false,
  });
  expect(levelGeometry.levelCount).toBeGreaterThan(0);
  expect(levelGeometry.levelCount).toBeLessThanOrEqual(7);
  expect(levelGeometry.minimumLevelWidth).toBeGreaterThanOrEqual(44);
  expect(levelGeometry.minimumLevelHeight).toBeGreaterThanOrEqual(44);
  await page.getByRole("button", { name: "Pilih level 1", exact: true }).click();
  const strainGeometry = await page.locator(".bid-strains").evaluate((group) => {
    const buttons = [...group.querySelectorAll<HTMLElement>("button")];
    return {
      count: buttons.length,
      minimumWidth: Math.min(...buttons.map((button) => button.getBoundingClientRect().width)),
      minimumHeight: Math.min(...buttons.map((button) => button.getBoundingClientRect().height)),
      overflowX: document.documentElement.scrollWidth > innerWidth,
      overflowY: document.documentElement.scrollHeight > innerHeight,
    };
  });
  expect(strainGeometry.count).toBeGreaterThan(0);
  expect(strainGeometry.minimumWidth).toBeGreaterThanOrEqual(44);
  expect(strainGeometry.minimumHeight).toBeGreaterThanOrEqual(44);
  expect(strainGeometry.overflowX).toBe(false);
  expect(strainGeometry.overflowY).toBe(false);
  await page.screenshot({ path: testInfo.outputPath("bidding-stage-320x700.png") });
  await page.getByRole("button", { name: "Bid 1NT", exact: true }).click();
  await expect(page.locator(".dummy-hand .physical-card")).toHaveCount(13);
  for (const [width, height] of [
    [320, 700],
    [375, 812],
    [390, 844],
    [768, 1024],
    [1024, 768],
    [1366, 768],
    [1440, 900],
    [1920, 1080],
  ]) {
    await page.setViewportSize({ width: width!, height: height! });
    const before = await page.locator(".own-hand").boundingBox();
    await expect(panel).toBeVisible();
    await panel.getByLabel("Pesan", { exact: true }).click();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    const chat = await panel.boundingBox();
    expect(chat!.height).toBeLessThan(height!);
    expect(chat!.x).toBeGreaterThanOrEqual(0);
    await page.screenshot({
      path: testInfo.outputPath(`table-chat-${width}.png`),
    });
    if (width! <= 390) {
      await page.setViewportSize({ width: width!, height: height! - 280 });
      await panel.getByLabel("Pesan", { exact: true }).fill("keyboard test");
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      await page.setViewportSize({ width: width!, height: height! });
    }
    const after = await page.locator(".own-hand").boundingBox();
    expect(Math.abs(before!.y - after!.y)).toBeLessThan(1);
  }
  await page.setViewportSize({ width: 1920, height: 1080 });
  const card = page.locator('button[aria-label^="Mainkan "]:enabled').last();
  await expect(card).toBeVisible();
  const count = await page
    .locator(".own-hand .physical-card,.dummy-hand .physical-card")
    .count();
  const start = await card.boundingBox();
  const target = await page.locator(".board-play-zone").boundingBox();
  expect(start).not.toBeNull();
  expect(target).not.toBeNull();
  await page.mouse.move(
    start!.x + start!.width / 2,
    start!.y + start!.height / 2,
  );
  await page.mouse.down();
  await expect(page.locator(".card-drag-preview")).toHaveCount(1);
  await page.mouse.move(
    target!.x + target!.width / 2,
    target!.y + target!.height / 2,
    { steps: 10 },
  );
  await page.mouse.up();
  await expect(
    page.locator(".own-hand .physical-card,.dummy-hand .physical-card"),
  ).toHaveCount(count - 1);
  await page.setViewportSize({ width: 390, height: 844 });
  const touchCard = page
    .locator('button[aria-label^="Mainkan "]:enabled')
    .last();
  await expect(touchCard).toBeVisible();
  await touchCard.scrollIntoViewIfNeeded();
  const touchStart = (await touchCard.boundingBox())!;
  const touchEnd = (await page.locator(".board-play-zone").boundingBox())!;
  const touchCount = await page
    .locator(".own-hand .physical-card,.dummy-hand .physical-card")
    .count();
  const cdp = await context.newCDPSession(page);
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [{ x: touchStart.x + 8, y: touchStart.y + 12 }],
  });
  await expect(page.locator(".card-drag-preview")).toHaveCount(1);
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchMove",
    touchPoints: [
      {
        x: touchEnd.x + touchEnd.width / 2,
        y: touchEnd.y + touchEnd.height / 2,
      },
    ],
  });
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchEnd",
    touchPoints: [],
  });
  await expect(
    page.locator(".own-hand .physical-card,.dummy-hand .physical-card"),
  ).toHaveCount(touchCount - 1);
  await cdp.detach();
  await context.close();
});

test("social toast actions and table chat stay scoped to authorized recipients", async ({
  browser,
}) => {
  test.setTimeout(120000);
  const first = await browser.newContext();
  const second = await browser.newContext();
  const third = await browser.newContext();
  const alice = await first.newPage();
  const bob = await second.newPage();
  const eve = await third.newPage();
  const suffix = Date.now();
  const aName = `sa_${suffix}`;
  const bName = `sb_${suffix}`;
  const a = await signup(alice, aName);
  const b = await signup(bob, bName);
  await signup(eve, `se_${suffix}`);
  await alice.evaluate(async (id) => {
    await fetch(`/api/account/users/${id}/follow`, { method: "PUT" });
  }, b.id);
  await expect(
    bob.getByText(`${aName} mulai mengikuti Anda`, { exact: true }),
  ).toBeVisible();
  await bob.getByRole("link", { name: "View User", exact: true }).click();
  await expect(bob.getByRole("searchbox")).toHaveValue(aName);
  await bob
    .getByRole("button", { name: `Follow ${aName}`, exact: true })
    .click();
  await expect(
    alice.getByText("Kalian sekarang berteman", { exact: true }),
  ).toBeVisible();
  await alice
    .locator(".chat-toast")
    .getByRole("button", { name: "Chat", exact: true })
    .click();
  await expect(
    alice.getByRole("region", { name: `Chat ${bName}` }),
  ).toBeVisible();
  await alice.goto("/lobby");
  await alice.getByRole("button", { name: "Buat meja", exact: true }).click();
  await expect(alice).toHaveURL(/\/table\//);
  await expect(alice.locator(".connection-status")).toContainText("Terhubung");
  const tableURL = alice.url();
  const tableId = tableURL.split("/").at(-1)!;
  await alice
    .getByRole("button", { name: "Invite player", exact: true })
    .click();
  await alice.getByRole("searchbox").fill(bName);
  await alice.getByRole("button", { name: "Invite", exact: true }).click();
  await expect(
    bob.getByText(`${aName} mengundang Anda bermain`, { exact: true }),
  ).toBeVisible();
  await bob.getByRole("link", { name: "Join", exact: true }).click();
  await bob.getByRole("button", { name: "Masuk", exact: true }).click();
  await expect(bob).toHaveURL(tableURL);
  await expect(bob.locator(".connection-status")).toContainText("Terhubung");
  await bob.goto("/friends");
  await expect(
    bob.getByRole("button", { name: `Unfollow ${aName}`, exact: true }),
  ).toBeVisible();
  await alice.getByRole("button", { name: "Tutup invite" }).click();
  await alice.getByLabel("Pesan", { exact: true }).fill("table only");
  await alice.getByRole("button", { name: "Kirim", exact: true }).click();
  await expect(bob.getByRole("button", { name: "Open Chat" })).toBeVisible();
  await bob.getByRole("button", { name: "Open Chat" }).click();
  await expect(bob.getByText("table only", { exact: true })).toHaveCount(1);
  await expect(eve.getByRole("button", { name: "Open Chat" })).toHaveCount(0);
  const status = await eve.evaluate(
    async (id) =>
      (await fetch(`/api/account/chat?scope=table&id=${id}`)).status,
    tableId,
  );
  expect(status).toBe(403);
  const privateStatus = await eve.evaluate(
    async (id) =>
      (await fetch(`/api/account/chat?scope=private&id=${id}`)).status,
    a.id,
  );
  expect(privateStatus).toBe(403);
  await expect
    .poll(() =>
      bob.evaluate(
        async () =>
          (await (await fetch("/api/account/invitations")).json()).length,
      ),
    )
    .toBe(0);
  await first.close();
  await second.close();
  await third.close();
});

test("history pagination preserves reading position and incoming messages show an indicator", async ({
  browser,
}) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const owner = await signup(page, `history_${Date.now()}`);
  const friend = {
    id: "11111111-1111-4111-8111-111111111111",
    username: "history_friend",
    displayName: "History Friend",
    avatar: "heart",
    online: true,
    following: true,
    friends: true,
  };
  await page.route("**/api/account/users?*", (route) =>
    route.fulfill({ json: [friend] }),
  );
  const makeMessage = (_index: number) => ({
    messageId: `history-${_index}`,
    scope: "private",
    conversationId: [owner.id, friend.id].sort().join(":"),
    senderUserId: friend.id,
    sender: friend,
    clientRequestId: `request-history-${_index}`,
    content: `Message ${_index}`,
    createdAt: new Date(Date.UTC(2026, 8, 10, 0, _index)).toISOString(),
  });
  await page.route("**/api/account/chat?*", (route) => {
    const older =
      new URL(route.request().url()).searchParams.get("cursor") === "older";
    return route.fulfill({
      json: {
        messages: Array.from({ length: 50 }, (_, _index) =>
          makeMessage((older ? 0 : 50) + _index),
        ).reverse(),
        ...(older ? {} : { nextCursor: "older" }),
      },
    });
  });
  let push: ((message: string) => void) | undefined;
  await page.routeWebSocket("ws://localhost:8180/v1/ws?*", (route) => {
    const server = route.connectToServer();
    server.onMessage((message) => route.send(message));
    push = (message) => route.send(message);
  });
  await page.goto("/friends");
  await page.getByRole("button", { name: "Chat", exact: true }).click();
  const panel = page.getByRole("region", { name: "Chat History Friend" });
  await expect(panel.getByText("Message 99", { exact: true })).toBeVisible();
  const list = panel.locator(".chat-messages");
  await list.evaluate((element) => {
    element.scrollTop = 0;
  });
  await expect(panel.getByText("Message 50", { exact: true })).toBeVisible();
  expect(push).toBeDefined();
  push!(
    JSON.stringify({
      v: 1,
      kind: "control",
      name: "chat.private.received",
      payload: { message: makeMessage(100) },
    }),
  );
  await expect(
    panel.getByRole("button", { name: "Pesan baru ↓" }),
  ).toBeVisible();
  expect(await list.evaluate((element) => element.scrollTop)).toBe(0);
  const before = await panel
    .getByText("Message 50", { exact: true })
    .boundingBox();
  await panel.getByRole("button", { name: "Muat riwayat" }).click();
  await expect(panel.getByText("Message 0", { exact: true })).toBeAttached();
  await expect
    .poll(async () =>
      Math.abs(
        (await panel.getByText("Message 50", { exact: true }).boundingBox())!
          .y - before!.y,
      ),
    )
    .toBeLessThan(2);
  await panel.getByRole("button", { name: "Pesan baru ↓" }).click();
  await expect(panel.getByText("Message 100", { exact: true })).toBeVisible();
  await context.close();
});
