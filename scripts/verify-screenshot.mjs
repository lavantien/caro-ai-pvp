import { createRequire } from 'node:module';
import {
	ARTIFACTS,
	BOARD,
	BROWSER,
	FRONTEND_DIR,
	FRONTEND_URL,
	SELECTORS,
	timeControl
} from './lib.mjs';

const require = createRequire(FRONTEND_DIR + '/package.json');
const chromium = require('playwright-core').chromium;

async function verify() {
  const browser = await chromium.launch({
    executablePath: chromium.executablePath(),
    headless: true,
    args: BROWSER.launchArgs,
  });
  const page = await browser.newPage({
    viewport: { width: BROWSER.viewportWidth, height: BROWSER.viewportHeight },
    deviceScaleFactor: BROWSER.deviceScaleFactor
  });

  const results = { passed: [], failed: [] };

  function check(name, condition, detail = '') {
    if (condition) {
      results.passed.push(name);
      console.log(`  PASS: ${name}`);
    } else {
      results.failed.push(`${name}${detail ? ': ' + detail : ''}`);
      console.log(`  FAIL: ${name}${detail ? ' - ' + detail : ''}`);
    }
  }

  try {
    console.log('\n=== Loading game page ===');
    await page.goto(`${FRONTEND_URL}/game`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(1000);

    console.log('\n=== Starting AIvAI game ===');
    await page.waitForSelector(SELECTORS.aiVsAiButton, { timeout: 10000 });
    await page.click(SELECTORS.aiVsAiButton);
    // 1+0 keeps per-move think time short so the first notation entry lands
    // well inside the wait below (7+5's opening think can run ~30s alone).
    await page.selectOption('select', timeControl('1+0').value);
    await page.click(SELECTORS.newGameButton);
    // Wait for AI moves to appear in notation
    await page.waitForFunction(
      (sel) => {
        const el = document.querySelector(sel);
        return el && el.textContent && !el.textContent.includes('No moves yet') && el.textContent.trim().length > 5;
      },
      SELECTORS.moveNotation,
      { timeout: 60000 }
    );

    // --- Board coordinate labels ---
    // Labels are <div class="... font-mono"> inside the grid, with text like "a", "b", ..., "p", "1", ..., "16"
    console.log('\n=== Board Coordinate Labels ===');
    const gridDiv = page.locator('.grid').first();
    const allLabelDivs = gridDiv.locator('div.text-gray-400.font-mono');
    const labelCount = await allLabelDivs.count();
    const labelTexts = [];
    for (let i = 0; i < labelCount; i++) {
      labelTexts.push((await allLabelDivs.nth(i).textContent()).trim());
    }

    const expectedCols = BOARD.columnLabels.split('');
    const expectedRows = Array.from({ length: BOARD.size }, (_, i) => String(i + 1));

    // Column labels appear top and bottom (2x)
    for (const col of expectedCols) {
      const count = labelTexts.filter(t => t === col).length;
      check(`Column "${col}" appears >= 2x`, count >= 2, `found ${count}x in [${labelTexts.join(',')}]`);
    }
    // Row labels appear left and right (2x)
    for (const row of expectedRows) {
      const count = labelTexts.filter(t => t === row).length;
      check(`Row "${row}" appears >= 2x`, count >= 2, `found ${count}x`);
    }

    // --- Board cells via data-x / data-y ---
    console.log('\n=== Board Grid ===');
    const cellButtons = page.locator('button[data-x][data-y]');
    const cellCount = await cellButtons.count();
    check(`Board has ${BOARD.totalCells} cells`, cellCount === BOARD.totalCells, `found ${cellCount}`);

    // --- AI labels in timer strips ---
    console.log('\n=== AI Difficulty Labels ===');
    // The label is in <span class="text-xs text-gray-500 font-medium"> after the player name
    const aiLabelSpans = page.locator('span.text-gray-500.font-medium');
    const aiLabelCount = await aiLabelSpans.count();
    const aiLabels = [];
    for (let i = 0; i < aiLabelCount; i++) {
      aiLabels.push((await aiLabelSpans.nth(i).textContent()).trim());
    }
    check('AI labels found (>= 2)', aiLabelCount >= 2, `labels: ${JSON.stringify(aiLabels)}`);
    // Check they contain "AI" and difficulty
    for (const lbl of aiLabels) {
      check(`AI label contains "AI"`, lbl.includes('AI'), `label: "${lbl}"`);
    }

    // --- Move notation ---
    console.log('\n=== Move Notation ===');
    const moveNotationEl = page.locator(SELECTORS.moveNotation);
    if (await moveNotationEl.count() > 0) {
      const notationText = await moveNotationEl.textContent();
      check('Move notation has moves', notationText && notationText.trim().length > 0, `text: "${notationText?.substring(0, 80)}"`);
      check('Notation uses simple format (no aa/bb/cc/dd)', !/\b(aa|bb|cc|dd)\d/.test(notationText), `text: "${notationText?.substring(0, 80)}"`);
      check('Notation has move numbers (N.letter)', /\d+\.[a-p]/.test(notationText), `text: "${notationText?.substring(0, 80)}"`);
    } else {
      check('Move notation element exists', false);
    }

    // --- Take verification screenshot ---
    console.log('\n=== Verification Screenshot ===');
    await page.screenshot({ path: ARTIFACTS.screenshotVerify, fullPage: true });
    check('Screenshot saved', true);

    // --- Summary ---
    console.log(`\n${'='.repeat(50)}`);
    console.log(`PASSED: ${results.passed.length}`);
    console.log(`FAILED: ${results.failed.length}`);
    if (results.failed.length > 0) {
      console.log('\nFailures:');
      results.failed.forEach(f => console.log(`  - ${f}`));
    }
    console.log(`${'='.repeat(50)}\n`);

  } catch (err) {
    console.error('Verification error:', err.message);
    console.error(err.stack);
  } finally {
    await browser.close();
  }

  return results.failed.length === 0;
}

verify().then(ok => process.exit(ok ? 0 : 1));
