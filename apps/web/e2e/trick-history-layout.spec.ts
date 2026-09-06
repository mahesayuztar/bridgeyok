import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

const stylesheet = readFileSync(join(__dirname, "../app/globals.css"), "utf8");

for (const viewport of [
  { width: 1440, height: 900 },
  { width: 768, height: 1024 },
  { width: 390, height: 844 },
  { width: 320, height: 568 },
  { width: 568, height: 320 },
]) {
  test(`trick history fits and centers at ${viewport.width}x${viewport.height}`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.setContent(`
      <style>${stylesheet.replace('@import "tailwindcss";', '')}</style>
      <button popovertarget="history">Buka riwayat trick</button>
      <section id="history" class="trick-history-popover" popover="auto" role="dialog">
        <header class="trick-history-header">
          <h2>Trick 1</h2>
          <button class="trick-history-close" popovertarget="history" popovertargetaction="hide">×</button>
        </header>
        <ol class="trick-history-list"><li class="trick-history-item"><div class="trick-history-cards">
          ${["top", "right", "bottom", "left"].map((position, _index) => `
            <div class="trick-history-play history-${position}">
              <span>${["N", "E", "S", "W"][_index]}</span>
              <span class="physical-card suit-s card-trick">
                <span class="card-corner"><strong>A</strong><span>♠</span></span>
                <span class="card-suit">♠</span>
              </span>
            </div>`).join("")}
        </div></li></ol>
        <nav class="trick-history-navigation"><button disabled>‹</button><button disabled>›</button></nav>
      </section>
    `);
    await page.getByRole("button", { name: "Buka riwayat trick" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    const geometry = await dialog.evaluate((element) => {
      const modal = element.getBoundingClientRect();
      const cards = [...element.querySelectorAll(".physical-card")].map((card) => card.getBoundingClientRect());
      const left = Math.min(...cards.map((card) => card.left));
      const right = Math.max(...cards.map((card) => card.right));
      return {
        modalCenter: Math.abs((modal.left + modal.right) / 2 - innerWidth / 2),
        verticalCenter: Math.abs((modal.top + modal.bottom) / 2 - innerHeight / 2),
        cardsCenter: Math.abs((left + right) / 2 - (modal.left + modal.right) / 2),
        inside: cards.every((card) => card.left >= modal.left && card.right <= modal.right && card.top >= modal.top && card.bottom <= modal.bottom),
        scrolls: [element, ...element.querySelectorAll(".trick-history-list, .trick-history-cards")].some((node) => node.scrollHeight > node.clientHeight + 1 || node.scrollWidth > node.clientWidth + 1),
      };
    });
    expect(geometry.modalCenter).toBeLessThanOrEqual(1);
    expect(geometry.verticalCenter).toBeLessThanOrEqual(1);
    expect(geometry.cardsCenter).toBeLessThanOrEqual(1);
    expect(geometry.inside).toBe(true);
    expect(geometry.scrolls).toBe(false);
    await page.screenshot({ path: testInfo.outputPath("trick-history.png") });
    await dialog.getByRole("button", { name: "×" }).click();
    await expect(dialog).toBeHidden();
  });
}
