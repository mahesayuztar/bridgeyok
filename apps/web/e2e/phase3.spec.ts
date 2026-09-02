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

async function dragPlayableCard(
  page: Page,
  pointer: "mouse" | "touch",
  selector = 'button[aria-label^="Mainkan "]:enabled',
) {
  const card = page.locator(selector).last();
  const handCards = page.locator(
    ".own-hand .physical-card, .dummy-hand .physical-card",
  );
  const board = page.locator(".board-play-zone");
  await card.scrollIntoViewIfNeeded();
  await expect(card).toBeVisible();
  const cardCountBefore = await handCards.count();
  const cardBox = await card.boundingBox();
  const boardBox = await board.boundingBox();
  expect(cardBox).not.toBeNull();
  expect(boardBox).not.toBeNull();
  const startX = cardBox!.x + cardBox!.width / 2;
  const startY = cardBox!.y + cardBox!.height / 2;
  const endX = boardBox!.x + boardBox!.width / 2;
  const endY = boardBox!.y + boardBox!.height / 2;
  const preview = page.locator(".card-drag-preview");

  if (pointer === "mouse") {
    await page.mouse.move(startX, startY);
    await page.mouse.down();
    await expect(preview).toHaveCount(1);
    await page.mouse.move(endX, endY, { steps: 5 });
    const previewBox = await preview.boundingBox();
    expect(previewBox).not.toBeNull();
    expect(previewBox!.x).toBeLessThan(endX);
    expect(previewBox!.x + previewBox!.width).toBeGreaterThan(endX);
    expect(previewBox!.y).toBeLessThan(endY);
    expect(previewBox!.y + previewBox!.height).toBeGreaterThan(endY);
    await page.mouse.up();
  } else {
    const session = await page.context().newCDPSession(page);
    await session.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x: startX, y: startY }],
    });
    await expect(page.locator('.physical-card[data-dragging="true"]')).toHaveCount(1);
    await expect(preview).toHaveCount(1);
    const pickupX = startX < endX ? startX + 24 : startX - 24;
    await session.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: pickupX, y: startY }],
    });
    for (let _step = 1; _step <= 5; _step++) {
      await session.send("Input.dispatchTouchEvent", {
        type: "touchMove",
        touchPoints: [
          {
            x: pickupX + ((endX - pickupX) * _step) / 5,
            y: startY + ((endY - startY) * _step) / 5,
          },
        ],
      });
      await page.waitForTimeout(20);
    }
    await expect(page.locator('.physical-card[data-dragging="true"]')).toHaveCount(1);
    const previewLayer = await preview.evaluate((element) => {
      const style = getComputedStyle(element);
      const board = document.querySelector(".board-play-zone");
      return {
        position: style.position,
        opacity: style.opacity,
        zIndex: Number(style.zIndex),
        boardZIndex:
          board === null ? 0 : Number(getComputedStyle(board).zIndex) || 0,
      };
    });
    expect(previewLayer.position).toBe("fixed");
    expect(previewLayer.opacity).toBe("1");
    expect(previewLayer.zIndex).toBeGreaterThan(previewLayer.boardZIndex);
    await session.send("Input.dispatchTouchEvent", {
      type: "touchEnd",
      touchPoints: [],
    });
    await session.detach();
  }

  await expect(handCards).toHaveCount(cardCountBefore - 1, { timeout: 250 });
}

async function returnCardFromInvalidDrop(page: Page) {
  const card = page.locator('button[aria-label^="Mainkan "]:enabled').last();
  const cardBox = await card.boundingBox();
  expect(cardBox).not.toBeNull();
  const cardCount = await page.locator(
    ".own-hand .physical-card, .dummy-hand .physical-card",
  ).count();
  await page.mouse.move(
    cardBox!.x + cardBox!.width / 2,
    cardBox!.y + cardBox!.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(8, 8, { steps: 5 });
  await page.mouse.up();
  await expect(page.locator(
    ".own-hand .physical-card, .dummy-hand .physical-card",
  )).toHaveCount(cardCount);
  await expect(card).toHaveAttribute("data-dragging", "false");
  await page.waitForTimeout(50);
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
    const ownHandElement = document.querySelector<HTMLElement>(".own-hand");
    const ownHandScrollElement =
      ownHandElement?.querySelector<HTMLElement>(".hand-cards") ?? null;
    const ownHand = rect(ownHandElement);
    const dummyCardElements = [
      ...document.querySelectorAll<HTMLElement>(".dummy-hand .physical-card"),
    ];
    const dummyCards = dummyCardElements.map(rect);
    const trickCards = [...document.querySelectorAll(".trick-slot .physical-card")].map(rect);
    const playedCardColors = [
      ...document.querySelectorAll<HTMLElement>(".trick-slot .physical-card"),
    ].map((card) => {
      const source = document.createElement("span");
      const suitClass = [...card.classList].find((name) => name.startsWith("suit-"));
      source.className = `physical-card card-hand ${suitClass ?? ""}`;
      source.style.position = "fixed";
      source.style.visibility = "hidden";
      document.body.append(source);
      const playedStyle = getComputedStyle(card);
      const sourceStyle = getComputedStyle(source);
      const comparison = {
        playedColor: playedStyle.color,
        sourceColor: sourceStyle.color,
        playedBackground: playedStyle.backgroundColor,
        sourceBackground: sourceStyle.backgroundColor,
        opacity: playedStyle.opacity,
        filter: playedStyle.filter,
      };
      source.remove();
      return comparison;
    });
    const dummyHand = rect(document.querySelector(".dummy-hand"));
    const currentTrick = rect(document.querySelector(".current-trick"));
    const cards = [...document.querySelectorAll(".physical-card")].map((card) => {
      const box = card.getBoundingClientRect();
      const ownHand = card.closest<HTMLElement>(".own-hand");
      const scrollContainer =
        card.closest<HTMLElement>(".bridge-hand")
          ?.querySelector<HTMLElement>(".hand-cards") ?? null;
      const zoneElement =
        ownHand?.querySelector<HTMLElement>(".hand-cards") ??
        card.closest<HTMLElement>(".board-play-zone");
      const zone = zoneElement?.getBoundingClientRect();
      const verticalClip =
        zone === undefined || box.top < zone.top - 1 || box.bottom > zone.bottom + 1;
      const horizontalClip =
        zone === undefined || box.left < zone.left - 1 || box.right > zone.right + 1;
      const horizontalScrollAvailable =
        scrollContainer !== null &&
        scrollContainer.scrollWidth > scrollContainer.clientWidth;
      return {
        ratio: box.width / box.height,
        width: box.width,
        cornerFontSize: Number.parseFloat(
          getComputedStyle(card.querySelector(".card-corner")!).fontSize,
        ),
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
        clipped: verticalClip || (horizontalClip && !horizontalScrollAvailable),
      };
    });
    const ownCardSlots = [
      ...document.querySelectorAll<HTMLElement>(".own-hand .hand-card-slot"),
    ];
    const dummyCardSlots = [
      ...document.querySelectorAll<HTMLElement>(".dummy-hand .hand-card-slot"),
    ];
    const dummySuitGroups = [
      ...document.querySelectorAll<HTMLElement>(".dummy-hand .dummy-suit-group"),
    ];
    const ownCardExposure = ownCardSlots.flatMap((slot, _slotIndex) => {
      const nextSlot = ownCardSlots[_slotIndex + 1];
      if (nextSlot === undefined) return [];
      const slotBox = slot.getBoundingClientRect();
      const nextBox = nextSlot.getBoundingClientRect();
      return Math.abs(slotBox.top - nextBox.top) <= 1
        ? [nextBox.left - slotBox.left]
        : [];
    });
    const ownCardRows = new Set(
      ownCardSlots.map((slot) => Math.round(slot.getBoundingClientRect().top)),
    ).size;
    const dummyCardRows = new Set(
      dummyCardSlots.map((slot) => Math.round(slot.getBoundingClientRect().top)),
    ).size;
    const dummyCardColumns = new Set(
      dummyCardSlots.map((slot) => Math.round(slot.getBoundingClientRect().left)),
    ).size;
    const dummySuitGroupRows = new Set(
      dummySuitGroups.map((group) => Math.round(group.getBoundingClientRect().top)),
    ).size;
    const dummySuitSpreads = dummySuitGroups.map((group) => {
      const groupCards = [
        ...group.querySelectorAll<HTMLElement>(".physical-card"),
      ].map(rect);
      const visibleCards = groupCards.filter(
        (card): card is NonNullable<typeof card> => card !== null,
      );
      if (visibleCards.length === 0) return 0;
      return (
        Math.max(...visibleCards.map((card) => card.right)) -
        Math.min(...visibleCards.map((card) => card.left))
      );
    });
    const playedCardsOverlap = trickCards.some((trickCard, _trickIndex) =>
      trickCards
        .slice(_trickIndex + 1)
        .some((otherTrickCard) => overlaps(trickCard, otherTrickCard)),
    );
    const wonIndicator = rect(document.querySelector(".trick-won"));
    const lostIndicator = rect(document.querySelector(".trick-lost"));
    return {
      cards,
      ownCardExposure,
      ownCardRows,
      ownHandScrollable:
        ownHandScrollElement !== null &&
        ownHandScrollElement.scrollWidth > ownHandScrollElement.clientWidth,
      dummyHasExtras:
        document.querySelector(".dummy-hand h3, .dummy-suit, .dummy-suit-label") !==
        null,
      dummySuitGroupCount: dummySuitGroups.length,
      dummySuitGroupRows,
      dummySuitOrder: dummySuitGroups.map((group) => group.dataset.suit),
      dummySuitSpreads,
      dummyPlacement:
        dummyHand === null || playZone === null
          ? null
          : {
              position: ["top", "right", "bottom", "left"].find((position) =>
                document
                  .querySelector(".dummy-hand")
                  ?.classList.contains(`dummy-${position}`),
              ),
              leftDelta: Math.abs(dummyHand.left - playZone.left),
              rightDelta: Math.abs(dummyHand.right - playZone.right),
              topDelta: Math.abs(dummyHand.top - playZone.top),
              bottomDelta: Math.abs(dummyHand.bottom - playZone.bottom),
              centerDelta: Math.abs(
                dummyHand.left + dummyHand.width / 2 -
                  (playZone.left + playZone.width / 2),
              ),
              width: dummyHand.width,
              height: dummyHand.height,
              cardRows: dummyCardRows,
              cardColumns: dummyCardColumns,
            },
      trickCenterDelta:
        currentTrick === null || playZone === null
          ? null
          : {
              x: Math.abs(
                currentTrick.left + currentTrick.width / 2 -
                  (playZone.left + playZone.width / 2),
              ),
              y: Math.abs(
                currentTrick.top + currentTrick.height / 2 -
                  (playZone.top + playZone.height / 2),
              ),
            },
      playedCardsOverlap,
      playedCardColors,
      dummyBlockedByTrick: dummyCardElements.some((card) => {
        const box = card.getBoundingClientRect();
        const target = document.elementFromPoint(box.left + 7, box.top + 7);
        return target !== null && target.closest(".current-trick") !== null;
      }),
      dummyTrickOverlap: dummyCards.some((dummyCard) =>
        trickCards.some((trickCard) => overlaps(dummyCard, trickCard)),
      ),
      indicator:
        wonIndicator === null || lostIndicator === null
          ? null
          : {
              won: wonIndicator,
              lost: lostIndicator,
              bottomDelta: Math.abs(wonIndicator.bottom - lostIndicator.bottom),
            },
      ownHandOverlapsSurface: overlaps(ownHand, surface),
      playZoneInsideSurface:
        surface !== null &&
        playZone !== null &&
        playZone.left >= surface.left &&
        playZone.right <= surface.right &&
        playZone.top >= surface.top &&
        playZone.bottom <= surface.bottom,
      sideParticipantWidths: [
        ...document.querySelectorAll(".player-left, .player-right"),
      ].flatMap((participant) => {
        const box = rect(participant);
        return box === null ? [] : [box.width];
      }),
      trickInsidePlayZone:
        currentTrick !== null &&
        playZone !== null &&
        currentTrick.left >= playZone.left - 1 &&
        currentTrick.right <= playZone.right + 1 &&
        currentTrick.top >= playZone.top - 1 &&
        currentTrick.bottom <= playZone.bottom + 1,
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
  expect(geometry.cards.every((card) => card.cornerFontSize >= 14)).toBe(true);
  expect(
    geometry.cards
      .filter((card) => card.className.includes("card-hand"))
      .every((card) => card.width >= 52),
  ).toBe(true);
  expect(
    geometry.cards
      .filter((card) => card.className.includes("card-dummy"))
      .every((card) => card.width >= 50),
  ).toBe(true);
  expect(
    geometry.cards
      .filter((card) => card.className.includes("card-trick"))
      .every((card) => card.width >= 49),
  ).toBe(true);
  expect(geometry.ownCardExposure.every((exposure) => exposure >= 18)).toBe(true);
  expect(geometry.ownCardRows).toBe(1);
  expect(geometry.ownHandScrollable).toBe(false);
  expect(geometry.dummyHasExtras).toBe(false);
  expect(geometry.playedCardsOverlap).toBe(false);
  expect(geometry.sideParticipantWidths.every((width) => width <= 31)).toBe(true);
  for (const card of geometry.playedCardColors) {
    expect(card).toEqual({
      playedColor: card.sourceColor,
      sourceColor: card.sourceColor,
      playedBackground: card.sourceBackground,
      sourceBackground: card.sourceBackground,
      opacity: "1",
      filter: "none",
    });
  }
  expect(geometry.trickCenterDelta).not.toBeNull();
  expect(geometry.trickInsidePlayZone).toBe(true);
  if (geometry.dummyPlacement?.position === "left") {
    expect(geometry.dummyPlacement.leftDelta).toBeLessThanOrEqual(8);
    expect(geometry.dummySuitGroupCount).toBe(4);
    expect(geometry.dummySuitGroupRows).toBe(4);
    expect(geometry.dummySuitOrder).toEqual(["C", "H", "S", "D"]);
    expect(Math.max(...geometry.dummySuitSpreads)).toBeGreaterThan(
      geometry.cards.find((card) => card.className.includes("card-dummy"))!.width,
    );
  } else if (geometry.dummyPlacement?.position === "right") {
    expect(geometry.dummyPlacement.rightDelta).toBeLessThanOrEqual(8);
    expect(geometry.dummySuitGroupCount).toBe(4);
    expect(geometry.dummySuitGroupRows).toBe(4);
    expect(geometry.dummySuitOrder).toEqual(["C", "H", "S", "D"]);
    expect(Math.max(...geometry.dummySuitSpreads)).toBeGreaterThan(
      geometry.cards.find((card) => card.className.includes("card-dummy"))!.width,
    );
  } else if (geometry.dummyPlacement?.position === "top") {
    expect(geometry.dummyPlacement.topDelta).toBeLessThanOrEqual(8);
    expect(geometry.dummyPlacement.width).toBeGreaterThan(
      geometry.dummyPlacement.height,
    );
    expect(geometry.dummyPlacement.cardRows).toBe(1);
    expect(geometry.dummySuitGroupCount).toBe(0);
  } else if (geometry.dummyPlacement?.position === "bottom") {
    expect(geometry.dummyPlacement.bottomDelta).toBeLessThanOrEqual(8);
    expect(geometry.dummyPlacement.width).toBeGreaterThan(
      geometry.dummyPlacement.height,
    );
    expect(geometry.dummyPlacement.cardRows).toBe(1);
    expect(geometry.dummySuitGroupCount).toBe(0);
  } else if (geometry.dummyPlacement !== null) {
    expect(geometry.dummyPlacement.centerDelta).toBeLessThanOrEqual(1);
  }
  expect(geometry.dummyBlockedByTrick).toBe(false);
  expect(geometry.dummyTrickOverlap).toBe(false);
  expect(geometry.ownHandOverlapsSurface).toBe(false);
  expect(geometry.playZoneInsideSurface).toBe(true);
  expect(geometry.documentOverflow).toEqual({ x: false, y: false });
  expect(geometry.indicator).not.toBeNull();
  expect(geometry.indicator!.won.height).toBeGreaterThan(
    geometry.indicator!.won.width,
  );
  expect(geometry.indicator!.lost.width).toBeGreaterThan(
    geometry.indicator!.lost.height,
  );
  expect(geometry.indicator!.bottomDelta).toBeLessThanOrEqual(1);
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
  test.setTimeout(300_000);
  const profiles = [
    { nickname: "Nara", viewport: { width: 1920, height: 1080 } },
    { nickname: "Eka", viewport: { width: 1024, height: 768 } },
    { nickname: "Sari", viewport: { width: 768, height: 1024 } },
    { nickname: "Wira", viewport: { width: 390, height: 844 } },
  ];
  const players = await Promise.all(profiles.map(async ({ nickname, viewport }) => {
    const context = await browser.newContext({
      viewport,
      hasTouch: viewport.width <= 390,
    });
    await context.addInitScript(() => {
      let ended: (() => void) | undefined;
      class TestAudioContext {
        currentTime = 0;
        destination = {};
        state = "running";
        createOscillator() {
          return {
            type: "sine",
            frequency: {
              setValueAtTime() {},
              exponentialRampToValueAtTime() {},
            },
            connect() {},
            start() {},
            stop() {
              const audioWindow = window as Window & { __turnCueCount?: number };
              audioWindow.__turnCueCount = (audioWindow.__turnCueCount ?? 0) + 1;
              queueMicrotask(() => ended?.());
            },
            addEventListener(_name: string, listener: () => void) {
              ended = listener;
            },
          };
        }
        createGain() {
          return {
            gain: {
              setValueAtTime() {},
              exponentialRampToValueAtTime() {},
            },
            connect() {},
          };
        }
        async resume() {}
        async close() {}
      }
      Object.defineProperty(window, "AudioContext", { value: TestAudioContext });
    });
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
    socket.on("framesent", (event) =>
      north.sentFrames.push(String(event.payload)),
    );
  });
  await replacementTab.goto(tableURL);
  await waitForConnection(replacementTab);
  await expect(replacementTab.getByRole("button", { name: "Kunci meja" })).toBeEnabled();
  await expect(replacementTab.getByRole("button", { name: "Ambil alih kendali" })).toHaveCount(0);
  await replacementTab.getByRole("button", { name: /^Buka menu Nara, kursi [NESW]$/ }).click();
  await replacementTab.getByRole("button", { name: "Saya siap" }).click();

  const activePages = [replacementTab, east.page, south.page, west.page];
  for (const page of activePages) {
    await expect(page.getByRole("button", { name: /^Buka menu Nara, kursi [NESW]$/ }).locator(".player-copy")).toContainText("Siap");
  }
  await south.page.emulateMedia({ reducedMotion: "reduce" });
  for (const [_playerIndex, page] of [east.page, south.page, west.page].entries()) {
    const nickname = ["Eka", "Sari", "Wira"][_playerIndex]!;
    await setReady(page, nickname);
    for (const activePage of activePages) {
      await expect(activePage.getByRole("button", { name: new RegExp(`^Buka menu ${nickname}, kursi [NESW]$`) }).locator(".player-copy")).toContainText("Siap");
    }
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
  await expect.poll(() => east.page.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  )).toBe(1);
  await east.page.getByLabel("Buka menu meja").click();
  await east.page.getByLabel("Suara giliran").uncheck();
  await expect.poll(() => east.page.evaluate(() =>
    window.localStorage.getItem("bridgeyok.turnAudioMuted"),
  )).toBe("true");
  await east.page.getByLabel("Buka menu meja").click();
  await makeCall(east.page, /^Pass/);
  await makeCall(south.page, /^Pass/);
  await west.page.reload();
  await waitForConnection(west.page);
  await expect(west.page.getByRole("button", { name: "Ambil alih kendali" })).toHaveCount(0);
  await makeCall(west.page, /^Pass/);
  await expect(
    east.page.locator('button[aria-label^="Mainkan "]:enabled').first(),
  ).toBeVisible();
  const unavailableCard = south.page.locator(
    'button[aria-label^="Mainkan "]:disabled',
  ).first();
  await expect(unavailableCard).toBeVisible();
  await unavailableCard.dispatchEvent("pointerdown", {
    pointerId: 91,
    pointerType: "mouse",
    isPrimary: true,
    button: 0,
  });
  await expect(south.page.locator(".card-drag-preview")).toHaveCount(0);
  const eastFramesBeforeDrag = mutationFrameCount(east.sentFrames);
  const declarerCueBeforeLead = await replacementTab.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  );
  const dummyCueBeforeLead = await south.page.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  );
  await returnCardFromInvalidDrop(east.page);
  expect(mutationFrameCount(east.sentFrames)).toBe(eastFramesBeforeDrag);
  await expect(
    east.page.locator('button[aria-label^="Mainkan "]:enabled').first(),
  ).toBeVisible();
  await east.page.evaluate(() => {
    const motionWindow = window as Window & { __motionStages?: string[] };
    const trick = document.querySelector(".current-trick");
    motionWindow.__motionStages = [];
    if (trick === null) return;
    new MutationObserver(() => {
      motionWindow.__motionStages?.push(
        trick.getAttribute("data-motion-stage") ?? "missing",
      );
    }).observe(trick, {
      attributeFilter: ["data-motion-stage"],
    });
  });
  await dragPlayableCard(east.page, "mouse");
  expect(mutationFrameCount(east.sentFrames)).toBe(eastFramesBeforeDrag + 1);
  await expect.poll(() => replacementTab.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  )).toBe(declarerCueBeforeLead + 1);
  await south.page.waitForTimeout(600);
  expect(await south.page.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  )).toBe(dummyCueBeforeLead);
  await expect.poll(() => east.page.evaluate(() =>
    (window as Window & { __motionStages?: string[] }).__motionStages?.includes(
      "moving",
    ) ?? false,
  )).toBe(true);
  const menuMotionStages = await east.page.getByLabel("Buka menu meja").evaluate(
    (element) => {
      const trick = document.querySelector(".current-trick");
      const before = trick?.getAttribute("data-motion-stage");
      (element as HTMLElement).click();
      return {
        before,
        after: trick?.getAttribute("data-motion-stage"),
      };
    },
  );
  expect(menuMotionStages.after).toBe(menuMotionStages.before);
  await east.page.getByLabel("Buka menu meja").evaluate((element) =>
    (element as HTMLElement).click(),
  );
  await expect.poll(() => east.page.evaluate(() =>
    (window as Window & { __turnCueCount?: number }).__turnCueCount ?? 0,
  )).toBe(1);
  for (const page of [north.page, replacementTab, east.page, south.page, west.page]) {
    if (page !== south.page) {
      await expect(page.locator(".dummy-hand .physical-card")).toHaveCount(13);
    }
    await expect(page.locator(".trick-slot .physical-card")).toHaveCount(1);
    await page.screenshot({
      path: testInfo.outputPath(
        `gameplay-${page.viewportSize()?.width}x${page.viewportSize()?.height}.png`,
      ),
      fullPage: false,
    });
    await assertGameplayGeometry(page);
  }

  const dummyPlay = replacementTab.locator(
    '.dummy-hand button[aria-label^="Mainkan "]:enabled',
  ).last();
  await expect(dummyPlay).toBeVisible();
  const dummyPlayLabel = await dummyPlay.getAttribute("aria-label");
  expect(dummyPlayLabel).not.toBeNull();
  const dummyFramesBeforeDrag = mutationFrameCount(north.sentFrames);
  await dragPlayableCard(
    replacementTab,
    "touch",
    '.dummy-hand button[aria-label^="Mainkan "]:enabled',
  );
  expect(mutationFrameCount(north.sentFrames)).toBe(dummyFramesBeforeDrag + 1);
  await expect(
    replacementTab.getByRole("button", {
      name: dummyPlayLabel!,
      exact: true,
    }),
  ).toHaveCount(0);
  const westFramesBeforeDrag = mutationFrameCount(west.sentFrames);
  await dragPlayableCard(west.page, "touch");
  expect(mutationFrameCount(west.sentFrames)).toBe(westFramesBeforeDrag + 1);
  await playNextCard(activePages);
  await expect(east.page.locator(".current-trick")).toHaveAttribute(
    "data-motion-stage",
    "winner",
  );
  for (const page of [replacementTab, east.page, south.page, west.page]) {
    await expect(page.locator(".trick-slot .physical-card")).toHaveCount(4);
    await assertGameplayGeometry(page);
    await page.screenshot({
      path: testInfo.outputPath(
        `gameplay-trick-${page.viewportSize()?.width}x${page.viewportSize()?.height}.png`,
      ),
      fullPage: false,
    });
  }
  const trickBox = await east.page.locator(".current-trick").boundingBox();
  expect(trickBox).not.toBeNull();
  await east.page.mouse.click(
    trickBox!.x + trickBox!.width / 2,
    trickBox!.y + trickBox!.height / 2,
  );
  await expect(east.page.locator(".current-trick")).not.toHaveAttribute(
    "data-motion-stage",
    "winner",
  );
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

  await east.page.locator(".board-result").click({ position: { x: 12, y: 12 } });
  await expect(east.page.locator(".board-result")).toHaveAttribute(
    "data-exiting",
    "true",
  );
  await expect(east.page.locator(".board-result")).toHaveCount(0);
  await expect(
    replacementTab.getByRole("button", { name: "Board berikutnya" }),
  ).toHaveCount(0);
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
