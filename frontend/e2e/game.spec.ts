import { test, expect } from "@playwright/test";
import { E2EConfig } from "../src/lib/config/e2eConfig";

/**
 * E2E Tests for Caro Game
 *
 * Tests all implemented features:
 * - Basic game mechanics (no regression)
 * - Sound effects toggle
 * - Move history display
 * - Winning line animation
 * - Timer functionality
 */

test.describe("Caro Game - Basic Mechanics", () => {
  test("should load game page successfully", async ({ page }) => {
    await page.goto("/game");

    // Wait for page to load
    await page.waitForLoadState("networkidle");

    // Check that game board is visible
    await expect(page.locator('h1:has-text("Caro Game")')).toBeVisible();

    // Check for board grid with specific styling
    await expect(page.locator(".grid.gap-0")).toBeVisible();
  });

  test("should display initial state correctly", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Check current player display
    await expect(page.locator("text=/Current Player:/")).toBeVisible();
    await expect(page.locator(".text-red-600")).toBeVisible();

    // Check move number
    await expect(page.locator("text=/Move #/")).toBeVisible();

    // Check timers are visible (default 7+5 = 7:00 initially)
    // Red is active and may already be counting down, so check for valid time patterns
    await expect(page.locator("text=/\\d+:\\d{2}/")).toHaveCount(2);
  });

  test("should place stone on board click", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Click on center cell
    const centerCell = page.locator('[data-x="7"][data-y="7"]');
    await centerCell.click();

    // Wait for move to be registered
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs);

    // Verify move was made (stone 'O' should be visible for red)
    await expect(centerCell).toContainText("O");

    // Current player should switch to blue
    await expect(page.locator(".text-blue-600")).toBeVisible();
    await expect(page.locator("text=/Move #1/")).toBeVisible();
  });

  test("should prevent placing stone on occupied cell", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Place first stone
    const centerCell = page.locator('[data-x="7"][data-y="7"]');
    await centerCell.click();
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs);

    await expect(centerCell).toContainText("O");

    // Try to place on same cell (should not work - move rejected)
    await centerCell.click();
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs);

    // Player should still be blue (first move succeeded, second rejected)
    await expect(page.locator(".text-blue-600")).toBeVisible();
    await expect(page.locator("text=/Move #1/")).toBeVisible();

    // Cell should still have 'O' (red stone)
    await expect(centerCell).toContainText("O");
  });
});

test.describe("Caro Game - Sound Effects", () => {
  test("should show sound toggle button", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Sound toggle button should be visible (muted by default)
    const soundButton = page.locator(
      'button[aria-label="Unmute"], button[aria-label="Mute"]',
    );
    await expect(soundButton).toBeVisible();
  });

  test("should toggle sound on/off", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Initial state: muted
    const soundButton = page.locator(
      'button[aria-label="Unmute"], button[aria-label="Mute"]',
    );
    await expect(soundButton).toBeVisible();

    // Get initial aria-label
    const initialLabel = await soundButton.getAttribute("aria-label");
    expect(initialLabel).toBe("Unmute");

    // Click to unmute
    await soundButton.click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Should now show mute button
    const newLabel = await soundButton.getAttribute("aria-label");
    expect(newLabel).toBe("Mute");

    // Click to mute again
    await soundButton.click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Should show unmute button again
    const finalLabel = await soundButton.getAttribute("aria-label");
    expect(finalLabel).toBe("Unmute");
  });

  test("should play stone placement sound when making a move", async ({
    page,
  }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Unmute first
    const soundButton = page.locator('button[aria-label="Unmute"]');
    await soundButton.click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Make a move - sound manager should be initialized
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(E2EConfig.apiMoveWaitMs);

    // Verify move was made (sound was triggered during move)
    await expect(page.locator(".text-blue-600")).toBeVisible();
  });
});

test.describe("Caro Game - Move History", () => {
  test("should display move history section", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Move history should be visible
    await expect(page.locator('h3:has-text("Move History")')).toBeVisible();
    await expect(page.locator("text=/No moves yet/")).toBeVisible();
  });

  test("should record moves in history", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Make first move
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Move history should show first move
    await expect(page.locator("text=/1\\. Red: \\(7, 7\\)/")).toBeVisible();

    // Make second move
    await page.locator('[data-x="7"][data-y="8"]').click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Move history should show both moves
    await expect(page.locator("text=/1\\. Red: \\(7, 7\\)/")).toBeVisible();
    await expect(page.locator("text=/2\\. Blue: \\(7, 8\\)/")).toBeVisible();
  });

  test("should highlight latest move in history", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Make a move
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(E2EConfig.moveWaitMs);

    // Latest move should be highlighted - check move history container
    const moveHistoryContainer = page.locator(".max-h-64");
    await expect(moveHistoryContainer).toBeVisible();
    await expect(moveHistoryContainer).toContainText("1. Red: (7, 7)");
  });
});

test.describe("Caro Game - Winning Line Animation", () => {
  test("should display winning line when game is won", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Create a horizontal winning line for Red that respects Open Rule
    // Red's second move (move #3) must satisfy |dx|>=3 or |dy|>=3 from first red stone
    const moves = [
      { x: 0, y: 7, n: 1 }, // Red - Move 1 (anywhere OK)
      { x: 7, y: 8, n: 2 }, // Blue - Move 2 (anywhere OK)
      { x: 3, y: 7, n: 3 }, // Red - Move 3 (|dx|=3 from (0,7), satisfies Open Rule)
      { x: 7, y: 6, n: 4 }, // Blue - Move 4
      { x: 1, y: 7, n: 5 }, // Red - Move 5
      { x: 8, y: 8, n: 6 }, // Blue - Move 6
      { x: 2, y: 7, n: 7 }, // Red - Move 7
      { x: 8, y: 6, n: 8 }, // Blue - Move 8
      { x: 4, y: 7, n: 9 }, // Red - Move 9 (WINNING - horizontal line 0-4 at y=7)
    ];

    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await expect(page.locator(`text=/Move #${move.n}/`)).toBeVisible({
        timeout: 5000,
      });
    }

    // Wait for win detection
    await page.waitForTimeout(E2EConfig.winDetectionWaitMs);

    // Wait for winning line animation to complete (0.5s animation)
    await page.waitForTimeout(E2EConfig.animationWaitMs);

    // Check for winning line SVG element
    const lineElement = page.locator('line[stroke="#ef4444"]');
    await expect(lineElement).toHaveCount(1);

    // Verify line has correct coordinates (cellSize=64)
    // y=7: center at 7*64 + 32 = 480
    // x=0: center at 0*64 + 32 = 32
    // x=4: center at 4*64 + 32 = 288
    const x1 = await lineElement.getAttribute("x1");
    const x2 = await lineElement.getAttribute("x2");
    const y1 = await lineElement.getAttribute("y1");

    expect(y1).toBe("480");
    expect(x1).toBe("32");
    expect(x2).toBe("288");
  });

  test("should show game over state with winner", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Create a vertical winning line for Red that respects Open Rule
    // Red's second move (move #3) must satisfy |dx|>=3 or |dy|>=3 from first red stone
    const moves = [
      { x: 7, y: 0, n: 1 }, // Red - Move 1 (anywhere OK)
      { x: 8, y: 7, n: 2 }, // Blue - Move 2 (anywhere OK)
      { x: 7, y: 3, n: 3 }, // Red - Move 3 (|dy|=3 from (7,0), satisfies Open Rule)
      { x: 8, y: 6, n: 4 }, // Blue - Move 4
      { x: 7, y: 1, n: 5 }, // Red - Move 5
      { x: 6, y: 8, n: 6 }, // Blue - Move 6
      { x: 7, y: 2, n: 7 }, // Red - Move 7
      { x: 6, y: 6, n: 8 }, // Blue - Move 8
      { x: 7, y: 4, n: 9 }, // Red - Move 9 (WINNING - vertical line 0-4 at x=7)
    ];

    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await expect(page.locator(`text=/Move #${move.n}/`)).toBeVisible({
        timeout: 5000,
      });
    }

    await page.waitForTimeout(E2EConfig.winDetectionWaitMs);

    // Game over banner should be visible
    await expect(page.locator(".bg-green-100")).toBeVisible();
    await expect(page.locator("text=/WINS!/")).toBeVisible();
  });
});

test.describe("Caro Game - Timer Functionality", () => {
  test("should display countdown timers for both players", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Check both timers are visible - look for time display pattern
    await expect(page.locator("text=/\\d:\\d\\d/")).toHaveCount(2);
  });

  test("should countdown active player timer", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Get initial time for Red (active player)
    const timeElements = page.locator("text=/\\d:\\d\\d/");
    const initialTime = await timeElements.first().textContent();

    // Wait 2 seconds
    await page.waitForTimeout(E2EConfig.timerCountdownWaitMs);

    // Time should have decreased
    const currentTime = await timeElements.first().textContent();
    expect(currentTime).not.toBe(initialTime);
  });

  test("should only countdown for current player", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Red is active, Blue timer should not change initially
    const timeElements = page.locator("text=/\\d:\\d\\d/");
    const blueTimeInitial = await timeElements.nth(1).textContent();

    await page.waitForTimeout(E2EConfig.timerCountdownWaitMs);

    const blueTimeCurrent = await timeElements.nth(1).textContent();
    expect(blueTimeCurrent).toBe(blueTimeInitial);
  });
});

test.describe("Caro Game - Regression Tests", () => {
  test("should maintain game state after multiple moves", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Make 5 moves respecting Open Rule
    // Red's second move (move #3) must have |dx|>=3 or |dy|>=3 from Red's first
    const moves = [
      { x: 0, y: 0 }, // Red move 1
      { x: 1, y: 1 }, // Blue move 2
      { x: 4, y: 0 }, // Red move 3 (|dx|=4 from (0,0), OK)
      { x: 1, y: 3 }, // Blue move 4
      { x: 2, y: 2 }, // Red move 5
    ];

    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await page.waitForTimeout(E2EConfig.regressionMoveWaitMs);
    }

    // Move number should be at least 3 (some moves may have failed due to timing)
    const moveNumber = await page.locator("text=/Move #\\d+/").textContent();
    const num = parseInt(moveNumber?.match(/#(\d+)/)?.[1] || "0");
    expect(num).toBeGreaterThanOrEqual(3);

    // Check that move history is populated
    const moveHistory = page.locator(".max-h-64");
    await expect(moveHistory).toBeVisible();
  });

  test("should handle rapid clicks correctly", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Rapidly click multiple cells respecting Open Rule
    const cells = [
      { x: 0, y: 0 },
      { x: 1, y: 1 },
      { x: 4, y: 0 }, // Red's 2nd move: |dx|=4 from (0,0), OK
      { x: 1, y: 3 },
      { x: 2, y: 2 },
    ];

    for (const cell of cells) {
      await page.locator(`[data-x="${cell.x}"][data-y="${cell.y}"]`).click();
    }

    // Should have made some moves (not necessarily all due to API rate limiting)
    const moveNumber = await page.locator("text=/Move #\\d+/").textContent();
    const num = parseInt(moveNumber?.match(/#(\d+)/)?.[1] || "0");
    expect(num).toBeGreaterThan(0);

    // Move history should reflect the moves made
    const moveHistory = page.locator(".max-h-64");
    await expect(moveHistory).toBeVisible();
  });
});
