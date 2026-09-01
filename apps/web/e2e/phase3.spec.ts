import { expect, test, type Page, type WebSocketRoute } from "@playwright/test";

function delayAuthoritativeGameplay(route: WebSocketRoute) {
  const server = route.connectToServer();
  server.onMessage((message) => {
    let kind: unknown;
    try {
      kind = JSON.parse(String(message)).kind;
    } catch {
      route.send(message);
      return;
    }
    if (kind === "ack" || kind === "event") {
      setTimeout(() => void route.send(message), 500);
    } else {
      route.send(message);
    }
  });
}

function mutationFrameCount(frames: string[]) {
  return frames.filter((encoded) => {
    try {
      const envelope = JSON.parse(encoded) as Record<string, unknown>;
      return envelope.kind === "command" && envelope.name !== "table.subscribe" && envelope.name !== "table.resume";
    } catch {
      return false;
    }
  }).length;
}

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
  await page.getByRole("button", { name: "Duduk" }).click();
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
  await expect(page.locator(".auction-table tbody")).toContainText(`${level}${strain}`, { timeout: 250 });
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
      await expect(cardButton).toHaveCount(0, { timeout: 250 });
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

async function assertGameplayGeometry(page: Page) {
  const geometry = await page.evaluate(() => {
    const rect = (element: Element | null) => {
      if (element === null) return null;
      const box = element.getBoundingClientRect();
      return {
        top: box.top,
        right: box.right,
        bottom: box.bottom,
        left: box.left,
        width: box.width,
        height: box.height,
      };
    };
    const overlaps = (
      first: ReturnType<typeof rect>,
      second: ReturnType<typeof rect>,
    ) =>
      first !== null &&
      second !== null &&
      first.left < second.right - 1 &&
      first.right > second.left + 1 &&
      first.top < second.bottom - 1 &&
      first.bottom > second.top + 1;
    const surface = rect(document.querySelector(".table-surface"));
    const playZone = rect(document.querySelector(".board-play-zone"));
    const ownHand = rect(document.querySelector(".own-hand"));
    const dummyCards = [...document.querySelectorAll(".dummy-hand .physical-card")].map(rect);
    const trickCards = [...document.querySelectorAll(".trick-slot .physical-card")].map(rect);
    const cards = [...document.querySelectorAll(".physical-card")].map((card) => {
      const box = card.getBoundingClientRect();
      const zone = card.closest(".board-play-zone, .own-hand")?.getBoundingClientRect();
      return {
        ratio: box.width / box.height,
        label: card.getAttribute("aria-label"),
        className: card.className,
        zoneClassName: card.closest(".board-play-zone, .own-hand")?.className,
        bounds:
          zone === undefined
            ? null
            : {
                top: box.top - zone.top,
                right: zone.right - box.right,
                bottom: zone.bottom - box.bottom,
                left: box.left - zone.left,
              },
        clipped:
          zone === undefined ||
          box.left < zone.left - 1 ||
          box.right > zone.right + 1 ||
          box.top < zone.top - 1 ||
          box.bottom > zone.bottom + 1,
      };
    });
    return {
      cards,
      dummyTrickOverlap: dummyCards.some((dummyCard) =>
        trickCards.some((trickCard) => overlaps(dummyCard, trickCard)),
      ),
      ownHandOverlapsSurface: overlaps(ownHand, surface),
      playZoneInsideSurface:
        surface !== null &&
        playZone !== null &&
        playZone.left >= surface.left &&
        playZone.right <= surface.right &&
        playZone.top >= surface.top &&
        playZone.bottom <= surface.bottom,
      viewport: { width: window.innerWidth, height: window.innerHeight },
      documentOverflow: {
        x: document.documentElement.scrollWidth > window.innerWidth,
        y: document.documentElement.scrollHeight > window.innerHeight,
      },
    };
  });
  expect(geometry.cards.length).toBeGreaterThan(0);
  expect(geometry.cards.every((card) => card.label !== null)).toBe(true);
  expect(
    geometry.cards.filter((card) => Math.abs(card.ratio - 5 / 7) >= 0.035),
  ).toEqual([]);
  expect(geometry.cards.filter((card) => card.clipped)).toEqual([]);
  expect(geometry.dummyTrickOverlap).toBe(false);
  expect(geometry.ownHandOverlapsSurface).toBe(false);
  expect(geometry.playZoneInsideSurface).toBe(true);
  expect(geometry.documentOverflow).toEqual({ x: false, y: false });
}

async function selectText(page: Page, selector: string) {
  return page.locator(selector).evaluate((element) => {
    const selection = window.getSelection();
    const range = document.createRange();
    range.selectNodeContents(element);
    selection?.removeAllRanges();
    selection?.addRange(range);
    return selection?.toString() ?? "";
  });
}

async function trickCounts(page: Page) {
  const indicator = page.locator(".trick-indicator");
  await expect(indicator).toBeVisible();
  return indicator.evaluate((element) => ({
    won: Number((element as HTMLElement).dataset.won),
    lost: Number((element as HTMLElement).dataset.lost),
    partnership: (element as HTMLElement).dataset.partnership,
  }));
}

test("four guests finish boards, recover a controller, and keep hidden hands private", async ({ browser }, testInfo) => {
  test.setTimeout(180_000);
  const profiles = [
    { nickname: "Nara", viewport: { width: 1920, height: 1080 } },
    { nickname: "Eka", viewport: { width: 1024, height: 768 } },
    { nickname: "Sari", viewport: { width: 768, height: 1024 } },
    { nickname: "Wira", viewport: { width: 390, height: 844 } },
  ];
  const players = await Promise.all(profiles.map(async ({ nickname, viewport }) => {
    const context = await browser.newContext({ viewport });
    const page = await context.newPage();
    const frames: string[] = [];
    const sentFrames: string[] = [];
    await page.routeWebSocket(/\/v1\/ws/, delayAuthoritativeGameplay);
    page.on("websocket", (socket) => {
      if (!socket.url().startsWith("ws://localhost:8180")) return;
      socket.on("framereceived", (event) =>
        frames.push(String(event.payload)),
      );
      socket.on("framesent", (event) =>
        sentFrames.push(String(event.payload)),
      );
    });
    await enterAsGuest(page, nickname);
    return { context, page, frames, sentFrames };
  }));
  const north = players[0]!;
  const east = players[1]!;
  const south = players[2]!;
  const west = players[3]!;

  await north.page.getByRole("button", { name: "Buat meja" }).click();
  await expect(north.page).toHaveURL(/\/table\/[0-9a-f-]+$/);
  await waitForConnection(north.page);
  const tableURL = north.page.url();
  const inviteCode = (await north.page.locator(".invite-inline .invite-code").textContent())?.trim();
  expect(inviteCode).toMatch(/^[A-Z2-7]{26}$/);
  expect(await selectText(north.page, ".invite-inline .invite-code")).toBe(inviteCode);
  await expect(north.page.getByRole("button", { name: /Salin/i })).toHaveCount(0);

  const botViewports = {
    N: { width: 1440, height: 900 },
    E: { width: 1024, height: 768 },
    S: { width: 390, height: 844 },
    W: { width: 320, height: 700 },
  } as const;
  for (const seat of ["N", "E", "S", "W"] as const) {
    await north.page.setViewportSize(botViewports[seat]);
    await north.page.getByRole("button", { name: `Buka menu kursi kosong ${seat}` }).click();
    await north.page.getByRole("button", { name: "Tambah bot" }).click();
    const botTrigger = north.page.getByRole("button", {
      name: `Buka menu Bot, kursi ${seat}, bot`,
    });
    await expect(botTrigger.locator(".bot-icon")).toHaveCount(1);
    await botTrigger.click();
    await expect(north.page.getByRole("dialog").getByRole("img", { name: "Bot" })).toBeVisible();
    await north.page.screenshot({
      path: testInfo.outputPath(
        `bot-${seat}-${botViewports[seat].width}x${botViewports[seat].height}.png`,
      ),
      fullPage: false,
    });
    await north.page.getByRole("button", { name: "Keluarkan bot" }).click();
    await expect(botTrigger).toHaveCount(0);
  }
  await north.page.setViewportSize({ width: 1920, height: 1080 });

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
  await replacementTab.setViewportSize({ width: 320, height: 700 });
  await replacementTab.routeWebSocket(/\/v1\/ws/, delayAuthoritativeGameplay);
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
  for (const [_playerIndex, page] of activePages.entries()) {
    await setReady(page, ["Nara", "Eka", "Sari", "Wira"][_playerIndex]!);
  }
  await expect(replacementTab.getByRole("button", { name: "Mulai board" })).toBeEnabled();
  await replacementTab.getByRole("button", { name: "Mulai board" }).click();
  await expect(
    replacementTab.locator('.auction-table th[data-turn="true"]'),
  ).toHaveText("N");
  await replacementTab.getByLabel("Buka menu meja").click();
  const activeInviteCode = replacementTab.locator(".table-menu .invite-code");
  await expect(activeInviteCode).toHaveText(inviteCode!);
  expect(await selectText(replacementTab, ".table-menu .invite-code")).toBe(inviteCode);
  await expect(replacementTab.getByRole("button", { name: /Salin/i })).toHaveCount(0);
  await replacementTab.getByLabel("Buka menu meja").click();
  for (const page of activePages) {
    await expect(page.locator(".own-hand .physical-card")).toHaveCount(13);
  }

  const invalidFrameCount = mutationFrameCount(east.sentFrames);
  await east.page.getByRole("button", { name: /^Pass/ }).dispatchEvent("click");
  await east.page.waitForTimeout(250);
  expect(mutationFrameCount(east.sentFrames)).toBe(invalidFrameCount);

  await makeBid(replacementTab, 1, "♣");
  await makeCall(east.page, /^Pass/);
  await makeCall(south.page, /^Pass/);
  await west.page.reload();
  await waitForConnection(west.page);
  await makeCall(west.page, /^Pass/);
  await expect(west.page.getByRole("button", { name: "Ambil alih kendali" })).toBeVisible();
  await west.page.getByRole("button", { name: "Ambil alih kendali" }).click();
  await makeCall(west.page, /^Pass/);

  await playNextCard(activePages);
  for (const page of [north.page, replacementTab, east.page, south.page, west.page]) {
    if (page !== south.page) {
      await expect(page.locator(".dummy-hand .physical-card")).toHaveCount(13);
    }
    await expect(page.locator(".trick-slot .physical-card")).toHaveCount(1);
    await assertGameplayGeometry(page);
    await page.screenshot({
      path: testInfo.outputPath(
        `gameplay-${page.viewportSize()?.width}x${page.viewportSize()?.height}.png`,
      ),
      fullPage: false,
    });
  }

  for (let _cardIndex = 1; _cardIndex < 4; _cardIndex++) {
    await playNextCard(activePages);
  }
  await expect.poll(async () => {
    const counts = await trickCounts(replacementTab);
    return counts.won + counts.lost;
  }).toBe(1);
  const [northTricks, eastTricks, southTricks, westTricks] = await Promise.all(
    [
      trickCounts(replacementTab),
      trickCounts(east.page),
      trickCounts(south.page),
      trickCounts(west.page),
    ],
  );
  expect(northTricks.partnership).toBe("NS");
  expect(southTricks).toEqual(northTricks);
  expect(eastTricks.partnership).toBe("EW");
  expect(westTricks).toEqual(eastTricks);
  expect(northTricks.won).toBe(eastTricks.lost);
  expect(northTricks.lost).toBe(eastTricks.won);
  expect(northTricks.won + northTricks.lost).toBe(1);
  const claimTrigger = replacementTab.getByLabel("Ajukan claim");
  await expect(claimTrigger).toBeVisible();
  await claimTrigger.focus();
  await replacementTab.keyboard.press("Enter");
  await expect(
    replacementTab.getByRole("group", {
      name: "Jumlah trick yang diklaim",
    }),
  ).toBeVisible();
  await expect(
    replacementTab.getByRole("button", { name: /^Claim \d+ trick$/ }).first(),
  ).toBeEnabled();
  await claimTrigger.click();
  await expect(claimTrigger).toBeFocused();
  await expect.poll(async () => {
    const availableUndo = await Promise.all(
      activePages.map((page) => page.getByLabel("Minta undo").count()),
    );
    return availableUndo.reduce((total, count) => total + count, 0);
  }).toBe(1);

  for (let _cardIndex = 4; _cardIndex < 52; _cardIndex++) {
    await playNextCard(activePages);
  }
  for (const page of activePages) {
    await expect(page.locator(".board-result")).toBeVisible();
  }
  for (const player of players) {
    assertPrivateFrames(player.frames);
  }

  await replacementTab.getByRole("button", { name: "Board berikutnya" }).click();
  await expect(
    replacementTab.locator('.auction-table th[data-turn="true"]'),
  ).toHaveText("E");
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
