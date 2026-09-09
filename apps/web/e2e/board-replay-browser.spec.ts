import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, resolve } from "node:path";
import ts from "typescript";
import { expect, test } from "@playwright/test";

function browserBundle(entry: string) {
  const modules = new Map<string, string>();
  function include(path: string): string {
    if (modules.has(path)) return path;
    modules.set(path, "");
    const source = readFileSync(path, "utf8");
    let code = /\.tsx?$/.test(path) ? ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2022 } }).outputText : source;
    code = code.replace(/require\(["']([^"']+)["']\)/g, (_, specifier: string) => {
      let dependency: string;
      try { dependency = createRequire(path).resolve(specifier); }
      catch {
        const base = resolve(dirname(path), specifier);
        try { dependency = createRequire(path).resolve(`${base}.ts`); }
        catch { dependency = createRequire(path).resolve(`${base}.tsx`); }
      }
      return `require(${JSON.stringify(include(dependency))})`;
    });
    modules.set(path, code);
    return path;
  }
  include(entry);
  return `(() => { const process = {env: {NODE_ENV: "development"}}; const modules = {${[...modules].map(([id, code]) => `${JSON.stringify(id)}: (module, exports, require) => {${code}\n}`).join(",")}}; const cache = {}; function require(id) { if (!cache[id]) { const module = {exports: {}}; cache[id] = module; modules[id](module, module.exports, require); } return cache[id].exports; } window.replayTest = require(${JSON.stringify(entry)}); })();`;
}

const root = resolve(__dirname, "..");
const bundle = browserBundle(resolve(__dirname, "fixtures/replay-harness.tsx"));
const stylesheet = readFileSync(resolve(root, "app/globals.css"), "utf8").replace('@import "tailwindcss";', "");

for (const viewport of [{ width: 1440, height: 900 }, { width: 768, height: 1024 }, { width: 390, height: 844 }, { width: 320, height: 700 }]) {
  test(`replay position, DDS fencing and table interaction at ${viewport.width}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.setContent(`<style>${stylesheet}</style><div id="root"></div>`);
    await page.addScriptTag({ content: bundle });
    const dialog = page.getByRole("dialog", { name: "Replay board 1", exact: true });
    await expect(dialog).toBeVisible();
    const geometry = await dialog.evaluate((element) => {
      const surface = element.querySelector(".board-play-zone")!.getBoundingClientRect();
      return { overflow: element.scrollWidth > element.clientWidth, outside: [...element.querySelectorAll(".completed-deal .physical-card")].filter((card) => {
        const rect = card.getBoundingClientRect();
        return rect.left < surface.left - 1 || rect.right > surface.right + 1 || rect.top < surface.top - 1 || rect.bottom > surface.bottom + 1;
      }).map((card) => ({ seat: card.closest("[data-seat]")?.getAttribute("data-seat"), rect: card.getBoundingClientRect().toJSON(), surface: surface.toJSON() })) };
    });
    await page.screenshot({ path: testInfo.outputPath("replay.png") });
    expect((await dialog.boundingBox())!.width).toBeGreaterThan((await dialog.boundingBox())!.height);
    const handle = dialog.locator("[data-dialog-drag-handle]");
    const initial = (await dialog.boundingBox())!;
    const handleBox = (await handle.boundingBox())!;
    await page.mouse.move(handleBox.x + 30, handleBox.y + 20);
    await page.mouse.down();
    await page.mouse.move(handleBox.x + 10, handleBox.y + 80, { steps: 5 });
    await page.mouse.up();
    expect((await dialog.boundingBox())!.y).toBeGreaterThan(initial.y + 40);
    await handle.focus();
    await page.keyboard.press("ArrowUp");
    expect((await dialog.boundingBox())!.y).toBeLessThan(initial.y + 60);
    expect(geometry).toEqual({ overflow: false, outside: [] });
    if (viewport.width <= 768) {
      expect((await dialog.boundingBox())!.height).toBeLessThan(viewport.height / 2);
      await page.getByRole("button", { name: "Bid on table" }).click();
      await expect(page.getByRole("button", { name: "Bid on table" })).toHaveText("Bid on table 1");
      await expect(dialog).toBeVisible();
      await page.locator(".own-hand button").last().click();
      await expect(page.locator(".own-hand .physical-card")).toHaveCount(12);
      await page.evaluate(() => (window as unknown as { replayTest: { resolveLivePosition: (step: number, tricks: number) => void } }).replayTest.resolveLivePosition(1, 10));
      await expect(page.locator(".own-hand .card-prediction").first()).toHaveText("10");
      await page.evaluate(() => (window as unknown as { replayTest: { resolveLivePosition: (step: number, tricks: number) => void } }).replayTest.resolveLivePosition(0, 2));
      await expect(page.locator(".own-hand .card-prediction").first()).toHaveText("10");
    }
    await dialog.getByRole("button", { name: "DD OFF", exact: true }).click();
    await expect(dialog.locator(".dds-spinner")).toHaveCount(13);
    await dialog.getByRole("button", { name: "Kartu berikutnya" }).click();
    await expect(dialog.locator(".completed-deal .physical-card")).toHaveCount(51);
    await expect(dialog.locator(".trick-slot")).toHaveCount(1);
    await page.evaluate(() => (window as unknown as { replayTest: { resolvePosition: (step: number, tricks: number) => void } }).replayTest.resolvePosition(1, 11));
    await expect(dialog.locator(".card-prediction").first()).toHaveText("11");
    await page.evaluate(() => (window as unknown as { replayTest: { resolvePosition: (step: number, tricks: number) => void } }).replayTest.resolvePosition(0, 2));
    await expect(dialog.locator(".card-prediction").first()).toHaveText("11");
    await expect(dialog.getByLabel("2 predicted tricks", { exact: true })).toHaveCount(0);
    await expect(dialog.locator(".card-prediction")).toHaveCount(3);
    await expect(dialog.locator(".suit-h .card-prediction, .suit-d .card-prediction, .suit-c .card-prediction")).toHaveCount(0);
    await dialog.getByRole("button", { name: "DD ON", exact: true }).click();
    await expect(dialog.locator(".card-prediction")).toHaveCount(0);
    await dialog.getByRole("button", { name: "DD OFF", exact: true }).click();
    await expect(dialog.locator(".dds-spinner")).toHaveCount(3);
    await page.evaluate(() => (window as unknown as { replayTest: { resolvePosition: (step: number, tricks: number) => void } }).replayTest.resolvePosition(1, 9));
    await expect(dialog.locator(".card-prediction").first()).toHaveText("9");
    for (let _step = 2; _step <= 4; _step++) {
      await dialog.getByRole("button", { name: "Kartu berikutnya" }).click();
      await expect(dialog.locator(".completed-deal .physical-card")).toHaveCount(52 - _step);
      await expect(dialog.locator(".trick-slot")).toHaveCount(_step);
    }
    await dialog.getByRole("button", { name: "Kartu sebelumnya" }).click();
    await expect(dialog.locator(".trick-slot")).toHaveCount(3);
    await dialog.getByRole("button", { name: "Kartu berikutnya" }).click();
    await dialog.getByRole("button", { name: "Kartu berikutnya" }).click();
    await expect(dialog.locator(".completed-deal .physical-card")).toHaveCount(52);
    await expect(dialog.getByLabel("Hasil board 1")).toBeVisible();
    await expect(dialog.getByRole("button", { name: "Kartu berikutnya" })).toBeDisabled();
    await expect(dialog.getByRole("navigation", { name: "Navigasi replay" })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await page.evaluate(() => (window as unknown as { replayTest: { renderHistory: () => void } }).replayTest.renderHistory());
    await page.getByRole("button", { name: "Buka skor meja" }).click();
    const scoreHistory = page.getByRole("dialog", { name: "History", exact: true });
    const replayTrigger = scoreHistory.getByRole("button", { name: "Replay board 1", exact: true });
    await replayTrigger.click();
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Tutup replay" }).click();
    await expect(dialog).toHaveCount(0);
    await expect(replayTrigger).toBeFocused();
  });
}

const modalBundle = browserBundle(resolve(__dirname, "fixtures/modal-harness.tsx"));
for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 700 }]) {
  test(`all dialogs drag with mouse and touch at ${viewport.width}`, async ({ page }) => {
    await page.setViewportSize(viewport);
    await page.setContent(`<style>${stylesheet}</style><div id="root"></div>`);
    await page.addScriptTag({ content: modalBundle });
    const touch = await page.context().newCDPSession(page);
    for (const [triggerName, title, closeName] of [
      ["Buka skor meja", "History", "Tutup skor meja"],
      ["Buka riwayat auction", "Auction", "Tutup riwayat auction"],
      [/Buka riwayat trick/, "Trick 1", "Tutup riwayat trick"],
      ["Keluar dari meja", "Keluar dari meja?", "Batal"],
      ["Ajukan claim", "Jumlah trick yang diklaim", "Claim 0 trick"],
      ["Receive claim", "Permintaan claim", "Tolak"],
      [/Buka menu South/, /South/, "Tutup menu pemain"],
    ] as const) {
      const trigger = triggerName === "Ajukan claim"
        ? page.getByLabel("Ajukan claim", { exact: true })
        : page.getByRole("button", { name: triggerName });
      await trigger.click();
      const dialog = page.getByRole("dialog", { name: title });
      await expect(dialog).toBeVisible();
      const handle = dialog.locator("[data-dialog-drag-handle]");
      const before = (await dialog.boundingBox())!;
      const header = (await handle.boundingBox())!;
      expect(before.x).toBeGreaterThanOrEqual(0);
      expect(before.x + before.width).toBeLessThanOrEqual(viewport.width);
      await page.mouse.move(header.x + 20, header.y + 12);
      await page.mouse.down();
      await page.mouse.move(header.x + 20, header.y - 28, { steps: 4 });
      await page.mouse.up();
      expect((await dialog.boundingBox())!.y).toBeLessThan(before.y);
      const start = (await handle.boundingBox())!;
      await touch.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: start.x + 20, y: start.y + 12 }] });
      await touch.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: start.x + 20, y: start.y + 62 }] });
      await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      expect((await dialog.boundingBox())!.y).toBeGreaterThan(before.y);
      await handle.focus();
      for (let _press = 0; _press < 40; _press++) await page.keyboard.press("Shift+ArrowLeft");
      expect((await dialog.boundingBox())!.x).toBeGreaterThanOrEqual(0);
      await dialog.getByRole("button", { name: closeName }).click();
      await expect(dialog).toBeHidden();
    }
  });
}

for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 568 }, { width: 568, height: 320 }]) {
  test(`replay mirrors canonical preview geometry at ${viewport.width}x${viewport.height}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.setContent(`<style>${stylesheet}</style><div id="root"></div>`);
    await page.addScriptTag({ content: bundle });
    const dialog = page.getByRole("dialog", { name: "Replay board 1", exact: true });
    await expect(dialog.locator(".completed-deal .physical-card")).toHaveCount(52);
    const deal = await page.evaluate(() => (window as unknown as { replayTest: { fullDeal: unknown } }).replayTest.fullDeal);
    await page.addScriptTag({ content: modalBundle });
    await page.evaluate((fullDeal) => {
      const surface = document.querySelector<HTMLElement>(".replay-surface-wrap > .table-surface")!;
      (window as unknown as { replayTest: { renderPreview: (deal: unknown, width: number, height: number) => void } }).replayTest.renderPreview(fullDeal, surface.offsetWidth, surface.offsetHeight);
    }, deal);
    await expect(page.locator("#canonical-preview .physical-card")).toHaveCount(52);
    const difference = await page.evaluate(() => {
      const preview = document.querySelector<HTMLElement>("#canonical-preview .table-surface")!;
      const mini = document.querySelector<HTMLElement>(".replay-surface-wrap .table-surface")!;
      preview.style.height = mini.style.height;
      const original = preview.getBoundingClientRect();
      const scaled = mini.getBoundingClientRect();
      const scale = scaled.width / original.width;
      const selectors = ".completed-deal .physical-card, .player-position";
      const cards = [...preview.querySelectorAll(selectors)];
      return Math.max(...[...mini.querySelectorAll(selectors)].flatMap((element, _index) => {
        const rect = element.getBoundingClientRect();
        const reference = cards[_index]!.getBoundingClientRect();
        return [Math.abs((rect.x - scaled.x) / scale - (reference.x - original.x)), Math.abs((rect.y - scaled.y) / scale - (reference.y - original.y)), Math.abs(rect.width / scale - reference.width), Math.abs(rect.height / scale - reference.height)];
      }));
    });
    expect(difference).toBeLessThan(0.1);
    const bounds = (await dialog.boundingBox())!;
    expect(bounds.width).toBeGreaterThan(bounds.height);
    expect(bounds.y + bounds.height).toBeLessThanOrEqual(viewport.height);
    const touch = await page.context().newCDPSession(page);
    const handle = (await dialog.locator("[data-dialog-drag-handle]").boundingBox())!;
    await touch.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: handle.x + 20, y: handle.y + 12 }] });
    await touch.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: 20, y: handle.y + 12 }] });
    await touch.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    expect((await dialog.boundingBox())!.x).toBeLessThanOrEqual(bounds.x);
    await page.screenshot({ path: testInfo.outputPath("mini-preview.png") });
  });
}

test("landscape touch devices can play behind replay and rotate after dragging", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 844, height: 390 }, hasTouch: true, isMobile: true });
  const page = await context.newPage();
  await page.setContent(`<meta name="viewport" content="width=device-width, initial-scale=1"><style>${stylesheet}</style><div id="root"></div>`);
  await page.addScriptTag({ content: bundle });
  const dialog = page.getByRole("dialog", { name: "Replay board 1", exact: true });
  await expect(dialog.locator(".physical-card")).toHaveCount(52);
  expect(await dialog.evaluate((element) => element.matches(":modal"))).toBe(false);
  await page.getByRole("button", { name: "Bid on table" }).tap();
  await expect(page.getByRole("button", { name: "Bid on table" })).toHaveText("Bid on table 1");
  const bounds = (await dialog.boundingBox())!;
  expect(bounds.width).toBeGreaterThan(bounds.height);
  const handle = dialog.locator("[data-dialog-drag-handle]");
  await handle.focus();
  await page.keyboard.press("Shift+ArrowRight");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(async () => {
    const rotated = (await dialog.boundingBox())!;
    return rotated.x >= 0 && rotated.x + rotated.width <= 390 && rotated.y + rotated.height <= 844;
  }).toBe(true);
  await expect(dialog).toBeVisible();
  await context.close();
});

for (const width of [320, 390]) {
  for (const position of ["left", "right"] as const) {
    test(`dense dummy stays clear of four trick cards at ${width} ${position}`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 844 });
      await page.setContent(`<style>${stylesheet}</style><div id="root"></div>`);
      await page.addScriptTag({ content: bundle });
      await page.evaluate(position => {
        (window as unknown as { replayTest: { renderDenseDummy: (position: "left" | "right") => void } }).replayTest.renderDenseDummy(position);
      }, position);
      await expect(page.getByRole("region", { name: "Dense dummy" }).locator(".physical-card")).toHaveCount(13);
      const overlaps = await page.evaluate(() => {
        const dummy = [...document.querySelectorAll(".dummy-hand .physical-card")].map(card => card.getBoundingClientRect());
        const trick = [...document.querySelectorAll(".trick-slot .physical-card")].map(card => card.getBoundingClientRect());
        return dummy.some(first => trick.some(second => first.left < second.right - 1 && first.right > second.left + 1 && first.top < second.bottom - 1 && first.bottom > second.top + 1));
      });
      await page.screenshot({ path: testInfo.outputPath("dense-dummy.png") });
      expect(overlaps).toBe(false);
    });
  }
}
