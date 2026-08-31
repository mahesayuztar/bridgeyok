import { expect, test, type Page } from "@playwright/test";

async function enterAsGuest(page: Page, nickname: string) {
  await page.goto("/");
  await page.getByLabel("Nama di meja").fill(nickname);
  await page.getByRole("button", { name: "Masuk sebagai tamu" }).click();
  await expect(page).toHaveURL(/\/lobby/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(nickname);
}

async function waitForConnection(page: Page) {
  await expect(page.locator(".connection-status")).toContainText("Terhubung");
}

async function takeSeat(page: Page, nickname: string, seat: "N" | "E" | "S" | "W") {
  const seatMenu = page.getByRole("button", {
    name: `Buka menu kursi kosong ${seat}`,
  });
  await seatMenu.click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await expect(seatMenu).toBeFocused();
  await seatMenu.click();
  await page.getByRole("button", { name: "Duduk di kursi" }).click();
  await expect(page.getByRole("button", { name: `Buka menu ${nickname}, kursi ${seat}` })).toBeVisible();
}

async function setReady(page: Page, nickname: string) {
  await page.getByRole("button", { name: new RegExp(`^Buka menu ${nickname}, kursi [NESW]$`) }).click();
  await page.getByRole("button", { name: "Saya siap" }).click();
  await expect(page.getByRole("button", { name: new RegExp(`^Buka menu ${nickname}, kursi [NESW]$`) }).locator(".player-copy")).toContainText("Siap");
}

async function makeCall(page: Page, name: RegExp) {
  const button = page.getByRole("button", { name });
  await expect(button).toBeEnabled();
  await button.click();
}

async function makeBid(page: Page, level: number, strain: string) {
  await page.getByRole("button", { name: String(level), exact: true }).click();
  const button = page.locator(".bid-strains button").filter({ hasText: strain });
  await expect(button).toBeEnabled();
  await button.click();
}

async function playNextCard(pages: Page[]) {
  await expect.poll(async () => {
    const counts = await Promise.all(pages.map((page) => page.locator('button[aria-label^="Mainkan "]:enabled').count()));
    return counts.reduce((total, count) => total + count, 0);
  }).toBeGreaterThan(0);
  for (const page of pages) {
    const cards = page.locator('button[aria-label^="Mainkan "]:enabled');
    if (await cards.count() > 0) {
      const card = cards.first();
      const cardLabel = await card.getAttribute("aria-label");
      const cardButton = page.getByRole("button", {
        name: cardLabel!,
        exact: true,
      });
      await card.dispatchEvent("click");
      await expect(cardButton).toHaveCount(0);
      return;
    }
  }
  throw new Error("no playable card was exposed to the current controller");
}

function assertPrivateFrames(frames: string[]) {
  for (const encoded of frames) {
    let envelope: Record<string, unknown>;
    try {
      envelope = JSON.parse(encoded) as Record<string, unknown>;
    } catch {
      continue;
    }
    const payload = envelope.payload as Record<string, unknown> | undefined;
    const table = envelope.kind === "snapshot" ? payload : payload?.table as Record<string, unknown> | undefined;
    const game = table?.game as Record<string, unknown> | undefined;
    if (game !== undefined && game.phase !== "BOARD_SCORED") {
      expect(game.fullDeal).toBeUndefined();
      if (game.dummyRevealed !== true) {
        expect(game.dummyHand).toBeUndefined();
      }
    }
    expect(encoded).not.toContain("sessionId");
    expect(encoded).not.toContain("deviceCredential");
    expect(encoded).not.toContain("accessToken");
  }
}

test("four guests finish boards, recover a controller, and keep hidden hands private", async ({ browser }) => {
  test.setTimeout(180_000);
  const players = await Promise.all(["Nara", "Eka", "Sari", "Wira"].map(async (nickname) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const frames: string[] = [];
    page.on("websocket", (socket) => {
      if (!socket.url().startsWith("ws://localhost:8180")) return;
      socket.on("framereceived", (event) =>
        frames.push(String(event.payload)),
      );
    });
    await enterAsGuest(page, nickname);
    return { context, page, frames };
  }));
  const north = players[0]!;
  const east = players[1]!;
  const south = players[2]!;
  const west = players[3]!;

  await north.page.getByRole("button", { name: "Buat meja" }).click();
  await expect(north.page).toHaveURL(/\/table\/[0-9a-f-]+$/);
  await waitForConnection(north.page);
  const tableURL = north.page.url();
  const inviteCode = (await north.page.locator(".invite-inline strong").textContent())?.trim();
  expect(inviteCode).toMatch(/^[A-Z2-7]{26}$/);

  for (const player of [east, south, west]) {
    await player.page.getByLabel("Kode undangan").fill(inviteCode!);
    await player.page.getByRole("button", { name: "Masuk" }).click();
    await expect(player.page).toHaveURL(/\/table\/[0-9a-f-]+$/);
    await waitForConnection(player.page);
  }
  await expect(north.page.getByText("4/4 pemain sudah masuk.")).toBeVisible();

  await takeSeat(north.page, "Nara", "N");
  await takeSeat(east.page, "Eka", "E");
  await takeSeat(south.page, "Sari", "S");
  await takeSeat(west.page, "Wira", "W");

  const replacementTab = await north.context.newPage();
  replacementTab.on("websocket", (socket) => {
    if (!socket.url().startsWith("ws://localhost:8180")) return;
    socket.on("framereceived", (event) =>
      north.frames.push(String(event.payload)),
    );
  });
  await replacementTab.goto(tableURL);
  await waitForConnection(replacementTab);
  await replacementTab.getByRole("button", { name: /^Buka menu Nara, kursi [NESW]$/ }).click();
  await replacementTab.getByRole("button", { name: "Saya siap" }).click();
  await expect(replacementTab.getByRole("button", { name: "Ambil alih kendali" })).toBeVisible();
  await replacementTab.getByRole("button", { name: "Ambil alih kendali" }).click();
  await expect(replacementTab.getByRole("button", { name: "Kunci meja" })).toBeEnabled();
  await north.page.getByRole("button", { name: "Kunci meja" }).click();
  await expect(north.page.getByRole("button", { name: "Ambil alih kendali" })).toBeVisible();

  const activePages = [replacementTab, east.page, south.page, west.page];
  for (const [playerIndex, page] of activePages.entries()) {
    await setReady(page, ["Nara", "Eka", "Sari", "Wira"][playerIndex]!);
  }
  await expect(replacementTab.getByRole("button", { name: "Mulai board" })).toBeEnabled();
  await replacementTab.getByRole("button", { name: "Mulai board" }).click();
  await expect(replacementTab.getByText("Dealer N")).toBeVisible();
  for (const page of activePages) {
    await expect(page.locator(".own-hand .physical-card")).toHaveCount(13);
  }

  await makeBid(replacementTab, 1, "♣");
  await makeCall(east.page, /^Pass/);
  await makeCall(south.page, /^Pass/);
  await west.page.reload();
  await waitForConnection(west.page);
  await makeCall(west.page, /^Pass/);
  await expect(west.page.getByRole("button", { name: "Ambil alih kendali" })).toBeVisible();
  await west.page.getByRole("button", { name: "Ambil alih kendali" }).click();
  await makeCall(west.page, /^Pass/);

  for (let _cardIndex = 0; _cardIndex < 52; _cardIndex++) {
    await playNextCard(activePages);
  }
  for (const page of activePages) {
    await expect(page.locator(".board-result")).toBeVisible();
  }
  for (const player of players) {
    assertPrivateFrames(player.frames);
  }

  await replacementTab.getByRole("button", { name: "Board berikutnya" }).click();
  await expect(replacementTab.getByText("Dealer E")).toBeVisible();
  await makeCall(east.page, /^Pass/);
  await makeCall(south.page, /^Pass/);
  await makeCall(west.page, /^Pass/);
  await makeCall(replacementTab, /^Pass/);
  await expect(replacementTab.getByText("Passed out").first()).toBeVisible();
  await replacementTab.getByRole("button", { name: "Akhiri meja" }).click();
  for (const page of activePages) {
    await expect(page.getByRole("heading", { name: "Terima kasih sudah bermain." })).toBeVisible();
  }

  await Promise.all(players.map((player) => player.context.close()));
});
