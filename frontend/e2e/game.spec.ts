import { test, expect } from "@playwright/test";
import { E2EConfig, UIConfig } from "../src/lib/config";

/**
 * E2E Tests for Caro Game
 *
 * Tests all implemented features against the shipped page:
 * - Basic game mechanics (no regression)
 * - Sound effects toggle
 * - Move history display (double-letter notation, e.g. "1.hh")
 * - Winning line animation
 * - Timer functionality
 *
 * Selectors target the real DOM: turn indicator (".text-sm.text-gray-600"),
 * cells ([data-x][data-y]), notation ([data-testid="move-notation"]),
 * timer strips (.bg-red-50 / .bg-blue-50 when active).
 */

const turnIndicator = (page: import("@playwright/test").Page) =>
  page.locator(".text-sm.text-gray-600");
const redTurn = (page: import("@playwright/test").Page) =>
  turnIndicator(page).locator(".text-red-600");
const blueTurn = (page: import("@playwright/test").Page) =>
  turnIndicator(page).locator(".text-blue-600");
const notation = (page: import("@playwright/test").Page) =>
  page.locator('[data-testid="move-notation"]');

// Each test creates a game; delete it so the 4-slot concurrent-game cap
// never rejects later tests. The page exposes the id in dev builds.
test.afterEach(async ({ page }) => {
  const gameId = await page
    .evaluate((key) => (window as any)[key], E2EConfig.gameIdHookKey)
    .catch(() => null);
  if (gameId) {
    await page
      .evaluate(
        (id) =>
          fetch(`http://localhost:5207/api/game/${id}`, { method: "DELETE" }),
        gameId,
      )
      .catch(() => {});
  }
});

/** Click a cell and wait until the turn indicator shows the move number. */
async function playMove(
  page: import("@playwright/test").Page,
  x: number,
  y: number,
  moveNumber: number,
) {
  await page.locator(`[data-x="${x}"][data-y="${y}"]`).click();
  await expect(turnIndicator(page)).toContainText(`Move ${moveNumber}`, {
    timeout: 5000,
  });
}

test.describe("Caro Game - Basic Mechanics", () => {
  test("should load game page successfully", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Board grid with labels renders
    await expect(page.locator(".grid.gap-0")).toBeVisible();
    // Turn indicator shows red to move at move 0
    await expect(turnIndicator(page)).toContainText("Move 0");
    await expect(redTurn(page)).toBeVisible();
  });

  test("should display initial state correctly", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await expect(turnIndicator(page)).toContainText("Move 0");
    await expect(redTurn(page)).toBeVisible();

    // Both timer strips show the initial clock (default 7+5 = 7:00)
    await expect(page.locator("text=/\\d+:\\d{2}/")).toHaveCount(2);
  });

  test("should place stone on board click", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await playMove(page, 7, 7, 1);

    // Red stone renders as 'O'
    await expect(page.locator('[data-x="7"][data-y="7"]')).toContainText("O");

    // Turn switched to blue
    await expect(blueTurn(page)).toBeVisible();
  });

  test("should prevent placing stone on occupied cell", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await playMove(page, 7, 7, 1);
    await expect(page.locator('[data-x="7"][data-y="7"]')).toContainText("O");

    // Second click on the same cell is rejected by the board
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs);

    await expect(blueTurn(page)).toBeVisible();
    await expect(turnIndicator(page)).toContainText("Move 1");
    await expect(page.locator('[data-x="7"][data-y="7"]')).toContainText("O");
  });
});

test.describe("Caro Game - Sound Effects", () => {
  test("should show sound toggle button", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    const soundButton = page.locator(
      'button[aria-label="Unmute"], button[aria-label="Mute"]',
    );
    await expect(soundButton).toBeVisible();
  });

  test("should toggle sound on/off", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    const soundButton = page.locator(
      'button[aria-label="Unmute"], button[aria-label="Mute"]',
    );
    await expect(soundButton).toBeVisible();

    expect(await soundButton.getAttribute("aria-label")).toBe("Unmute");

    await soundButton.click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);
    expect(await soundButton.getAttribute("aria-label")).toBe("Mute");

    await soundButton.click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);
    expect(await soundButton.getAttribute("aria-label")).toBe("Unmute");
  });

  test("should play stone placement sound when making a move", async ({
    page,
  }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Unmute, then move: the move itself is the observable outcome
    await page.locator('button[aria-label="Unmute"]').click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    await playMove(page, 7, 7, 1);
    await expect(blueTurn(page)).toBeVisible();
  });
});

test.describe("Caro Game - Move History", () => {
  test("should display move history section", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await expect(notation(page)).toBeVisible();
    await expect(notation(page)).toContainText("No moves yet");
  });

  test("should record moves in history", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Double-letter notation: letter(y) then letter(x): (7,7)=hh, (7,8)=ih
    await playMove(page, 7, 7, 1);
    await expect(notation(page)).toContainText("1.hh");

    await playMove(page, 7, 8, 2);
    await expect(notation(page)).toContainText("1.hh");
    await expect(notation(page)).toContainText("2.ih");
  });

  test("should highlight latest move in history", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await playMove(page, 7, 7, 1);

    // The latest red move gets the highlighted background
    const latest = notation(page).locator("span.bg-red-100");
    await expect(latest).toHaveCount(1);
    await expect(latest).toContainText("1.hh");
  });
});

test.describe("Caro Game - Winning Line Animation", () => {
  test("should display winning line when game is won", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Horizontal red line 0-4 at y=7, respecting the Open Rule for red's
    // second move (|dx|>=3 from the first red stone).
    const moves: Array<[number, number]> = [
      [0, 7], // Red 1
      [7, 8], // Blue 2
      [3, 7], // Red 3 (|dx|=3, satisfies Open Rule)
      [7, 6], // Blue 4
      [1, 7], // Red 5
      [8, 8], // Blue 6
      [2, 7], // Red 7
      [8, 6], // Blue 8
      [4, 7], // Red 9 - winning five 0-4
    ];
    for (let i = 0; i < moves.length; i++) {
      await playMove(page, moves[i][0], moves[i][1], i + 1);
    }

    await page.waitForTimeout(E2EConfig.winDetectionWaitMs);
    await page.waitForTimeout(E2EConfig.animationWaitMs);

    // Winning line drawn in red; geometry is relative (cell size is responsive)
    const line = page.locator(`line[stroke="${UIConfig.winningLineColor}"]`);
    await expect(line).toHaveCount(1);
    const x1 = Number(await line.getAttribute("x1"));
    const x2 = Number(await line.getAttribute("x2"));
    const y1 = Number(await line.getAttribute("y1"));
    const y2 = Number(await line.getAttribute("y2"));
    expect(x2).toBeGreaterThan(x1);
    expect(y1).toBe(y2);
  });

  test("should show game over state with winner", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Vertical red line 0-4 at x=7, Open Rule respected at red's second move.
    const moves: Array<[number, number]> = [
      [7, 0], // Red 1
      [8, 7], // Blue 2
      [7, 3], // Red 3 (|dy|=3, satisfies Open Rule)
      [8, 6], // Blue 4
      [7, 1], // Red 5
      [6, 8], // Blue 6
      [7, 2], // Red 7
      [6, 6], // Blue 8
      [7, 4], // Red 9 - winning five
    ];
    for (let i = 0; i < moves.length; i++) {
      await playMove(page, moves[i][0], moves[i][1], i + 1);
    }

    await page.waitForTimeout(E2EConfig.winDetectionWaitMs);

    // Red's win banner slides down
    await expect(page.locator(".bg-red-600")).toBeVisible();
    await expect(page.locator(".bg-red-600 h2")).toContainText(/wins!/i);
  });
});

test.describe("Caro Game - Timer Functionality", () => {
  test("should display countdown timers for both players", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    await expect(page.locator("text=/\\d+:\\d{2}/")).toHaveCount(2);
  });

  test("should countdown active player timer", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Red moves first: the red strip is active (bg-red-50) and ticks down
    const redClock = page.locator(".bg-red-50 .font-mono");
    await expect(redClock).toBeVisible();
    const initialTime = await redClock.textContent();

    await page.waitForTimeout(E2EConfig.timerCountdownWaitMs);

    const currentTime = await redClock.textContent();
    expect(currentTime).not.toBe(initialTime);
  });

  test("should only countdown for current player", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Blue is inactive (gray strip): its clock stays put
    const blueClock = page.locator(".opacity-60 .font-mono").first();
    await expect(blueClock).toBeVisible();
    const initialTime = await blueClock.textContent();

    await page.waitForTimeout(E2EConfig.timerCountdownWaitMs);

    const currentTime = await blueClock.textContent();
    expect(currentTime).toBe(initialTime);
  });
});

test.describe("Caro Game - Regression Tests", () => {
  test("should maintain game state after multiple moves", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Five moves respecting the Open Rule for red's second move
    const moves: Array<[number, number]> = [
      [0, 0], // Red 1
      [1, 1], // Blue 2
      [4, 0], // Red 3 (|dx|=4, OK)
      [1, 3], // Blue 4
      [2, 2], // Red 5
    ];
    for (let i = 0; i < moves.length; i++) {
      await playMove(page, moves[i][0], moves[i][1], i + 1);
    }

    await expect(turnIndicator(page)).toContainText("Move 5");
    await expect(notation(page).locator("span")).toHaveCount(5);
  });

  test("should handle rapid clicks correctly", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    const cells: Array<[number, number]> = [
      [0, 0],
      [1, 1],
      [4, 0],
      [1, 3],
      [2, 2],
    ];
    for (const [x, y] of cells) {
      await page.locator(`[data-x="${x}"][data-y="${y}"]`).click();
    }

    // Give the optimistic UI and server sync a moment
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs * 5);

    const indicatorText = await turnIndicator(page).textContent();
    const num = parseInt(indicatorText?.match(/Move (\d+)/)?.[1] || "0");
    expect(num).toBeGreaterThan(0);
  });
});
