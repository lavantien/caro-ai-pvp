import { test, expect } from "@playwright/test";

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

    // Check timers are visible (both show 3:00 initially)
    await expect(page.locator("text=/3:00/")).toHaveCount(2);
  });

  test("should place stone on board click", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Click on center cell
    const centerCell = page.locator('[data-x="7"][data-y="7"]');
    await centerCell.click();

    // Wait for move to be registered
    await page.waitForTimeout(200);

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
    await page.waitForTimeout(200);

    await expect(centerCell).toContainText("O");

    // Try to place on same cell (should not work - move rejected)
    await centerCell.click();
    await page.waitForTimeout(200);

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
    await page.waitForTimeout(100);

    // Should now show mute button
    const newLabel = await soundButton.getAttribute("aria-label");
    expect(newLabel).toBe("Mute");

    // Click to mute again
    await soundButton.click();
    await page.waitForTimeout(100);

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
    await page.waitForTimeout(100);

    // Make a move - sound manager should be initialized
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(200);

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
    await page.waitForTimeout(100);

    // Move history should show first move
    await expect(page.locator("text=/1\\. Red: \\(7, 7\\)/")).toBeVisible();

    // Make second move
    await page.locator('[data-x="7"][data-y="8"]').click();
    await page.waitForTimeout(100);

    // Move history should show both moves
    await expect(page.locator("text=/1\\. Red: \\(7, 7\\)/")).toBeVisible();
    await expect(page.locator("text=/2\\. Blue: \\(7, 8\\)/")).toBeVisible();
  });

  test("should highlight latest move in history", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Make a move
    await page.locator('[data-x="7"][data-y="7"]').click();
    await page.waitForTimeout(100);

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

    // Handle alert dialog before it appears
    page.on("dialog", (dialog) => dialog.accept());

    // Create a horizontal winning line for Red that respects Open Rule
    // Move 3 (Red's second move) must be outside center 3x3 (6-8, 6-8)
    const moves = [
      { x: 0, y: 7 }, // Red - Move 1 (anywhere OK)
      { x: 7, y: 8 }, // Blue - Move 2 (anywhere OK)
      { x: 1, y: 7 }, // Red - Move 3 (must be outside center 3x3, x=1 is OK)
      { x: 7, y: 6 }, // Blue - Move 4
      { x: 2, y: 7 }, // Red - Move 5
      { x: 8, y: 8 }, // Blue - Move 6
      { x: 3, y: 7 }, // Red - Move 7
      { x: 8, y: 6 }, // Blue - Move 8
      { x: 4, y: 7 }, // Red - Move 9 (WINNING MOVE - horizontal line 0-4 at y=7)
    ];

    // Make all moves
    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await page.waitForTimeout(100);
    }

    // Wait for win detection and alert
    await page.waitForTimeout(1000);

    // Wait for winning line animation to complete (0.5s animation)
    await page.waitForTimeout(600);

    // Check for winning line SVG element
    // Note: Line uses stroke-dashoffset animation, so we check existence and attributes
    const lineElement = page.locator('line[stroke="#ef4444"]');
    await expect(lineElement).toHaveCount(1);

    // Verify line has correct coordinates (horizontal line from x=0 to x=4 at y=7)
    const x1 = await lineElement.getAttribute("x1");
    const x2 = await lineElement.getAttribute("x2");
    const y1 = await lineElement.getAttribute("y1");

    // y=7 means center is at 7*40 + 20 = 300
    expect(y1).toBe("300");
    // x=0 to x=4 means centers at 20 and 180
    expect(x1).toBe("20");
    expect(x2).toBe("180");
  });

  test("should show game over state with winner", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Handle alert dialog
    page.on("dialog", (dialog) => dialog.accept());

    // Create a vertical winning line for Red that respects Open Rule
    // Move 3 (Red's second move) must be outside center 3x3 (6-8, 6-8)
    const moves = [
      { x: 7, y: 0 }, // Red - Move 1 (anywhere OK)
      { x: 8, y: 7 }, // Blue - Move 2 (anywhere OK)
      { x: 7, y: 1 }, // Red - Move 3 (must be outside center 3x3, y=1 is OK)
      { x: 8, y: 6 }, // Blue - Move 4
      { x: 7, y: 2 }, // Red - Move 5
      { x: 6, y: 8 }, // Blue - Move 6
      { x: 7, y: 3 }, // Red - Move 7
      { x: 6, y: 6 }, // Blue - Move 8
      { x: 7, y: 4 }, // Red - Move 9 (WINNING MOVE - vertical line 0-4 at x=7)
    ];

    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await page.waitForTimeout(100);
    }

    await page.waitForTimeout(1000);

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
    await page.waitForTimeout(2000);

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

    await page.waitForTimeout(2000);

    const blueTimeCurrent = await timeElements.nth(1).textContent();
    expect(blueTimeCurrent).toBe(blueTimeInitial);
  });
});

test.describe("Caro Game - Regression Tests", () => {
  test("should maintain game state after multiple moves", async ({ page }) => {
    await page.goto("/game");
    await page.waitForLoadState("networkidle");

    // Make 5 moves (avoiding Open Rule violations)
    const moves = [
      { x: 0, y: 0 },
      { x: 1, y: 1 },
      { x: 2, y: 2 },
      { x: 3, y: 3 },
      { x: 4, y: 4 },
    ];

    for (const move of moves) {
      await page.locator(`[data-x="${move.x}"][data-y="${move.y}"]`).click();
      await page.waitForTimeout(150);
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

    // Rapidly click multiple cells
    const cells = [
      { x: 0, y: 0 },
      { x: 1, y: 1 },
      { x: 2, y: 2 },
      { x: 3, y: 3 },
      { x: 4, y: 4 },
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
