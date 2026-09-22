import { expect, test, type Page, type WebSocket as PlaywrightWebSocket } from "@playwright/test";

type FrameMetadata = {
  direction: "sent" | "received";
  connectionId: number;
  kind?: string;
  name?: string;
  tableId?: string;
  requestId?: string;
  revision?: number;
  seq?: number;
  pendingCount: number;
};

type SessionTelemetry = {
  appSocketOpenCount: number;
  appSocketCloseCount: number;
  ignoredWebSocketCount: number;
  tableGetCount: number;
  frames: FrameMetadata[];
  pendingRequestIds: Set<string>;
};

function isAppSocket(socket: PlaywrightWebSocket) {
  return new URL(socket.url()).pathname === "/v1/ws";
}

function observeSession(page: Page): SessionTelemetry {
  const telemetry: SessionTelemetry = {
    appSocketOpenCount: 0,
    appSocketCloseCount: 0,
    ignoredWebSocketCount: 0,
    tableGetCount: 0,
    frames: [],
    pendingRequestIds: new Set(),
  };

  page.on("request", request => {
    const url = new URL(request.url());
    if (request.method() === "GET" && /^\/v1\/tables\/[^/]+$/.test(url.pathname)) {
      telemetry.tableGetCount += 1;
    }
  });
  page.on("websocket", socket => {
    if (!isAppSocket(socket)) {
      telemetry.ignoredWebSocketCount += 1;
      return;
    }
    const connectionId = ++telemetry.appSocketOpenCount;
    socket.on("close", () => {
      telemetry.appSocketCloseCount += 1;
    });
    socket.on("framesent", event => recordFrame(telemetry, connectionId, "sent", String(event.payload)));
    socket.on("framereceived", event => recordFrame(telemetry, connectionId, "received", String(event.payload)));
  });
  return telemetry;
}

function recordFrame(
  telemetry: SessionTelemetry,
  connectionId: number,
  direction: FrameMetadata["direction"],
  encoded: string,
) {
  let frame: Record<string, unknown>;
  try {
    frame = JSON.parse(encoded) as Record<string, unknown>;
  } catch {
    return;
  }
  const kind = typeof frame.kind === "string" ? frame.kind : undefined;
  const name = typeof frame.name === "string" ? frame.name : undefined;
  const requestId = typeof frame.request_id === "string" ? frame.request_id : undefined;
  if (direction === "sent" && kind === "command" && requestId && name !== "table.subscribe" && name !== "table.resume") {
    telemetry.pendingRequestIds.add(requestId);
  }
  if (direction === "received" && requestId && (kind === "ack" || kind === "error")) {
    telemetry.pendingRequestIds.delete(requestId);
  }
  telemetry.frames.push({
    direction,
    connectionId,
    ...(kind ? { kind } : {}),
    ...(name ? { name } : {}),
    ...(typeof frame.table_id === "string" ? { tableId: frame.table_id } : {}),
    ...(requestId ? { requestId } : {}),
    ...(typeof frame.revision === "number" ? { revision: frame.revision } : {}),
    ...(typeof frame.seq === "number" ? { seq: frame.seq } : {}),
    pendingCount: telemetry.pendingRequestIds.size,
  });
}

function marker(telemetry: SessionTelemetry) {
  return {
    appSocketOpenCount: telemetry.appSocketOpenCount,
    appSocketCloseCount: telemetry.appSocketCloseCount,
    tableGetCount: telemetry.tableGetCount,
    frameIndex: telemetry.frames.length,
  };
}

function delta(telemetry: SessionTelemetry, baseline: ReturnType<typeof marker>) {
  const frames = telemetry.frames.slice(baseline.frameIndex);
  return {
    appSocketOpenCount: telemetry.appSocketOpenCount - baseline.appSocketOpenCount,
    appSocketCloseCount: telemetry.appSocketCloseCount - baseline.appSocketCloseCount,
    tableGetCount: telemetry.tableGetCount - baseline.tableGetCount,
    subscribeCount: frames.filter(frame => frame.direction === "sent" && frame.name === "table.subscribe").length,
    resumeCount: frames.filter(frame => frame.direction === "sent" && frame.name === "table.resume").length,
    takeoverCount: frames.filter(frame => frame.direction === "sent" && frame.name === "table.takeover").length,
  };
}

async function waitForTableConnection(page: Page) {
  await expect(page.locator(".connection-status")).toContainText("Terhubung");
}

async function softNavigate(page: Page, path: string) {
  await page.evaluate(nextPath => {
    const router = (window as unknown as { next?: { router?: { push: (href: string) => void } } }).next?.router;
    if (!router) throw new Error("Next router instrumentation is unavailable");
    router.push(nextPath);
  }, path);
}

test("table to workspace navigation preserves the shared game session", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const telemetry = observeSession(page);
  await page.goto("/signup");
  await page.getByLabel("Username", { exact: true }).fill(`p6_${Date.now()}`);
  await page.getByLabel("Nama di meja").fill("P6 Baseline");
  await page.getByLabel("Kata sandi", { exact: true }).fill("bridge test password");
  await page.getByRole("button", { name: "Sign Up", exact: true }).click();
  await expect(page).toHaveURL(/\/play$/);
  await page.getByRole("link", { name: /Casual Game/ }).click();
  await expect(page).toHaveURL(/\/lobby$/);
  await page.getByRole("button", { name: "Buat meja" }).click();
  await expect(page).toHaveURL(/\/table\//);
  await waitForTableConnection(page);
  await page.getByRole("button", { name: "Buka menu kursi kosong N" }).click();
  await page.getByRole("button", { name: "Duduk", exact: true }).click();
  await expect(page.getByRole("button", { name: "Buka menu P6 Baseline, kursi N" })).toBeVisible();
  await expect.poll(() => telemetry.pendingRequestIds.size).toBe(0);

  const tablePath = new URL(page.url()).pathname;
  const tableMountId = await page.locator(".persistent-table-workspace").evaluate(element => {
    const mountId = crypto.randomUUID();
    element.setAttribute("data-test-mount-id", mountId);
    return mountId;
  });

  const switchBaseline = marker(telemetry);
  const failedSwitchTableId = "00000000-0000-4000-8000-000000000001";
  await page.route(`**/v1/tables/${failedSwitchTableId}`, async route => {
    const currentTableUrl = new URL(route.request().url());
    currentTableUrl.pathname = `/v1/tables/${tablePath.split("/").at(-1)}`;
    const response = await route.fetch({ url: currentTableUrl.href });
    const projection = await response.json() as Record<string, unknown>;
    await route.fulfill({ response, json: { ...projection, tableId: failedSwitchTableId } });
  });
  await page.route(`**/v1/tables/${tablePath.split("/").at(-1)}/leave`, route => route.fulfill({
    status: 503,
    contentType: "application/problem+json",
    body: JSON.stringify({ code: "SERVICE_UNAVAILABLE", retryable: true }),
  }));
  await softNavigate(page, `/table/${failedSwitchTableId}`);
  await expect(page.getByRole("heading", { name: "Pindah meja?" })).toBeVisible();
  await page.getByRole("button", { name: "Tinggalkan dan pindah" }).click();
  await expect(page.locator(".workspace-switch .issue-notice")).toContainText("Layanan meja sedang bermasalah");
  await expect.poll(() => delta(telemetry, switchBaseline).appSocketOpenCount).toBe(0);
  await expect.poll(() => delta(telemetry, switchBaseline).appSocketCloseCount).toBe(0);
  expect(delta(telemetry, switchBaseline).subscribeCount).toBe(0);
  expect(delta(telemetry, switchBaseline).resumeCount).toBe(0);
  expect(delta(telemetry, switchBaseline).takeoverCount).toBe(0);
  await page.unrouteAll({ behavior: "wait" });
  await page.getByRole("button", { name: "Tetap di meja ini" }).click();
  await expect(page).toHaveURL(new RegExp(`${tablePath}$`));
  await waitForTableConnection(page);

  await softNavigate(page, "/friends");
  await expect(page.getByRole("heading", { name: "Friends", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Tutup workspace dan kembali ke meja" }).click();
  await expect(page).toHaveURL(new RegExp(`${tablePath}$`));
  await waitForTableConnection(page);

  const navigationBaseline = marker(telemetry);
  await softNavigate(page, "/friends");
  await expect(page).toHaveURL(/\/friends$/);
  await expect(page.getByRole("heading", { name: "Friends", exact: true })).toBeVisible();
  await page.getByLabel("Cari pemain").fill("state tetap");
  await page.getByLabel("Friends saja").uncheck();
  await expect(page.locator(`[data-test-mount-id="${tableMountId}"]`)).toBeAttached();
  await expect(page.locator(`[data-test-mount-id="${tableMountId}"]`)).toBeHidden();
  await testInfo.attach("friends-workspace-desktop", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect(page.getByRole("button", { name: "Tutup workspace dan kembali ke meja" })).toBeVisible();
  await testInfo.attach("friends-workspace-mobile", { body: await page.screenshot({ fullPage: true }), contentType: "image/png" });
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect.poll(() => delta(telemetry, navigationBaseline).appSocketOpenCount).toBe(0);
  await expect.poll(() => delta(telemetry, navigationBaseline).appSocketCloseCount).toBe(0);

  await page.getByRole("navigation", { name: "Navigasi utama" }).getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("link", { name: "Settings dan profile", exact: true }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  await page.getByLabel("Nama di meja").fill("P6 Updated");
  await softNavigate(page, "/history");
  await expect(page.getByRole("heading", { name: "History", exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Meja aktif", exact: true })).toBeVisible();
  await page.goBack();
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  await expect(page.getByLabel("Nama di meja")).toHaveValue("P6 Updated");
  await page.goBack();
  await expect(page.getByRole("heading", { name: "Friends", exact: true })).toBeVisible();
  await expect(page.getByLabel("Cari pemain")).toHaveValue("state tetap");
  await expect(page.getByLabel("Friends saja")).not.toBeChecked();
  await page.goForward();
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Simpan profile" }).click();
  await expect(page.getByRole("status")).toContainText("Profile tersimpan.");
  expect(delta(telemetry, navigationBaseline).appSocketOpenCount).toBe(0);
  expect(delta(telemetry, navigationBaseline).appSocketCloseCount).toBe(0);

  await softNavigate(page, tablePath);
  await expect(page).toHaveURL(/\/table\//);
  await waitForTableConnection(page);
  await expect(page.locator(`[data-test-mount-id="${tableMountId}"]`)).toBeVisible();
  await expect.poll(() => delta(telemetry, navigationBaseline)).toEqual({
    appSocketOpenCount: 0,
    appSocketCloseCount: 0,
    tableGetCount: 0,
    subscribeCount: 0,
    resumeCount: 0,
    takeoverCount: 0,
  });
  await expect.poll(() => telemetry.pendingRequestIds.size).toBe(0);
  const navigationDelta = delta(telemetry, navigationBaseline);

  await softNavigate(page, "/settings");
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  const restoreBaseline = marker(telemetry);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Settings", exact: true })).toBeVisible();
  await expect.poll(() => page.locator(".persistent-table-workspace").count()).toBe(1);
  await expect.poll(() => delta(telemetry, restoreBaseline)).toEqual({
    appSocketOpenCount: 1,
    appSocketCloseCount: 0,
    tableGetCount: 1,
    subscribeCount: 0,
    resumeCount: 1,
    takeoverCount: 1,
  });
  await expect.poll(() => telemetry.pendingRequestIds.size).toBe(0);
  const restoreDelta = delta(telemetry, restoreBaseline);

  await softNavigate(page, tablePath);
  await waitForTableConnection(page);

  const leaveBaseline = marker(telemetry);
  await page.getByRole("button", { name: "Keluar", exact: true }).click();
  await expect(page).toHaveURL(/\/lobby$/);
  await expect(page.getByRole("heading", { name: /^Halo,/ })).toBeVisible();
  await expect.poll(() => delta(telemetry, leaveBaseline)).toEqual({
    appSocketOpenCount: 1,
    appSocketCloseCount: 1,
    tableGetCount: 0,
    subscribeCount: 0,
    resumeCount: 0,
    takeoverCount: 0,
  });
  const leaveDelta = delta(telemetry, leaveBaseline);

  const accountNavigationBaseline = marker(telemetry);
  await softNavigate(page, "/friends");
  await expect(page.getByRole("heading", { name: "Friends", exact: true })).toBeVisible();
  await expect.poll(() => delta(telemetry, accountNavigationBaseline)).toEqual({
    appSocketOpenCount: 0,
    appSocketCloseCount: 0,
    tableGetCount: 0,
    subscribeCount: 0,
    resumeCount: 0,
    takeoverCount: 0,
  });

  const report = {
    route: { tablePath, workspacePath: "/friends", settingsPath: "/settings" },
    hmrAndOtherWebSocketsIgnored: telemetry.ignoredWebSocketCount,
    navigation: navigationDelta,
    restore: restoreDelta,
    leaveToAccountRealtime: leaveDelta,
    final: {
      appSocketOpenCount: telemetry.appSocketOpenCount,
      appSocketCloseCount: telemetry.appSocketCloseCount,
      tableGetCount: telemetry.tableGetCount,
      pendingCount: telemetry.pendingRequestIds.size,
      maxRevision: Math.max(0, ...telemetry.frames.map(frame => frame.revision ?? 0)),
      maxSeq: Math.max(0, ...telemetry.frames.map(frame => frame.seq ?? 0)),
    },
    frames: telemetry.frames,
  };
  await testInfo.attach("phase6-session-baseline", {
    body: Buffer.from(JSON.stringify(report, null, 2)),
    contentType: "application/json",
  });
});
