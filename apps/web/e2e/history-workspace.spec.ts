import { expect, test } from "@playwright/test";

const participants = {
  north: { id: "00000000-0000-4000-8000-000000000001", nickname: "North Player", isBot: false },
  east: { id: "00000000-0000-4000-8000-000000000002", nickname: "East Player", isBot: false },
  south: { id: "00000000-0000-4000-8000-000000000003", nickname: "South Player", isBot: false },
  west: { id: "00000000-0000-4000-8000-000000000004", nickname: "West Player", isBot: false },
};

const ranks = ["A", "K", "Q", "J", "T", "9", "8", "7", "6", "5", "4", "3", "2"];
const suits = ["S", "H", "D", "C"];
const fullDeal = Object.fromEntries(["north", "east", "south", "west"].map((hand, _handIndex) => [
  hand,
  suits.flatMap((suit, _suitIndex) => ranks.filter((_, _rankIndex) => (_rankIndex + _suitIndex) % 4 === _handIndex).map((rank) => ({ suit, rank }))),
]));

function historyBoard(boardId: string, boardNumber: number, completedAt: string, label?: string, replayAllowed = false) {
  const result = {
    passedOut: false,
    contract: { level: 3, strain: "NT", doubling: "UNDOUBLED", declarer: "N" },
    tricksDeclarer: 9,
    tricksNS: 9,
    tricksEW: 4,
    vulnerability: "NONE",
    scoreNS: 400,
  };
  return {
    boardId,
    tableId: "00000000-0000-4000-8000-000000000010",
    boardNumber,
    result,
    lineup: {
      seats: { N: participants.north, E: participants.east, S: participants.south, W: participants.west },
      northSouth: { id: "pair-ns", members: [participants.north, participants.south] },
      eastWest: { id: "pair-ew", members: [participants.east, participants.west] },
    },
    viewerSeat: "N",
    completedAt,
    ...(label === undefined ? {} : { label }),
    replayAllowed,
  };
}

test("history master-detail remains usable through search, label editing and mobile widths", async ({ page }) => {
  const boards = [
    historyBoard("00000000-0000-4000-8000-000000000011", 2, "2026-10-04T10:00:00Z", "review later", true),
    historyBoard("00000000-0000-4000-8000-000000000012", 1, "2026-10-03T10:00:00Z"),
  ];
  const username = `wi08_${Date.now()}`;

  await page.route("**/api/account/history/boards*", async (route) => {
    await route.fulfill({ json: { items: boards } });
  });
  await page.route("**/api/account/history/boards/*/label", async (route) => {
    if (route.request().method() === "PUT") {
      await route.fulfill({ json: { label: "reviewed" } });
      return;
    }
    await route.fulfill({ status: 204 });
  });
  await page.route("**/v1/boards/*/replay", async (route) => {
    await route.fulfill({ json: {
      boardId: "00000000-0000-4000-8000-000000000011",
      fullDeal,
      game: {
        rulesetVersion: "bridgeyok_duplicate_v1",
        phase: "BOARD_SCORED",
        dummyRevealed: false,
        board: { number: 2, dealer: "N", vulnerability: "NONE" },
        auction: { dealer: "N", complete: true, passedOut: true, calls: ["N", "E", "S", "W"].map((seat) => ({ seat, call: { kind: "PASS" } })) },
        completedTricks: [],
        currentTrick: { plays: [] },
        tricksNS: 0,
        tricksEW: 0,
        claimed: false,
        result: { passedOut: true, vulnerability: "NONE", tricksNS: 0, tricksEW: 0, tricksDeclarer: 0, scoreNS: 0 },
      },
    } });
  });

  await page.goto("/signup");
  await page.getByLabel("Username", { exact: true }).fill(username);
  await page.getByLabel("Nama di meja").fill("WI08 Player");
  await page.getByLabel("Kata sandi", { exact: true }).fill("bridge test password");
  await page.getByRole("button", { name: "Sign Up", exact: true }).click();
  await expect(page).toHaveURL(/\/play$/);
  await page.getByRole("link", { name: "History", exact: true }).click();
  await expect(page.getByTestId("history-workspace")).toBeVisible();
  await expect(page.getByTestId("history-board-select")).toHaveCount(2);

  await page.getByTestId("history-board-select").nth(1).click();
  await expect(page.getByTestId("history-board-select").nth(1)).toHaveAttribute("aria-pressed", "true");
  await page.getByLabel("Cari berdasarkan tanggal, label, atau contract").fill("review later");
  await expect(page.getByTestId("history-board-select")).toHaveCount(1);
  await expect(page.getByTestId("history-board-select")).toHaveAttribute("aria-pressed", "true");
  await page.getByLabel("Cari berdasarkan tanggal, label, atau contract").fill("");
  await expect(page.getByTestId("history-board-select")).toHaveCount(2);
  await page.getByTestId("history-board-select").first().click();

  const label = page.getByLabel("Label belajar board 2");
  await label.fill("reviewed");
  await label.press("Enter");
  await expect(page.getByRole("status")).toContainText("Label board tersimpan.");
  await expect(label).toHaveValue("reviewed");
  await page.getByTestId("history-replay-button").click();
  await expect(page.getByRole("dialog", { name: "Replay board 2" })).toBeVisible();
  await page.getByRole("button", { name: "Tutup replay" }).click();
  await expect(page.getByTestId("history-replay-button")).toBeFocused();

  for (const viewport of [{ width: 320, height: 800 }, { width: 390, height: 844 }, { width: 768, height: 1024 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: test.info().outputPath(`history-${viewport.width}.png`), fullPage: true });
  }
});
