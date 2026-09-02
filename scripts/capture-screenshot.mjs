#!/usr/bin/env node

/**
 * E2E Screenshot Capture Script
 *
 * Runs the full pipeline (backend + frontend), plays an AI vs AI match
 * through the real UI, captures a screenshot of the game-winning position,
 * and inserts it into README.md.
 *
 * Usage: node scripts/capture-screenshot.mjs
 */

import { createRequire } from 'node:module';
import { readFileSync, writeFileSync, createWriteStream } from 'node:fs';
import {
	ARTIFACTS,
	BROWSER,
	FRONTEND_DIR,
	FRONTEND_URL,
	SCREENSHOT,
	SELECTORS,
	TIMEOUTS,
	createProcessManager,
	startBackend,
	teeConsole,
	timeControl,
	waitForUrl
} from './lib.mjs';

const logStream = createWriteStream(ARTIFACTS.e2eLog, { flags: 'w' });
teeConsole(logStream);

const mgr = createProcessManager();

// Resolve playwright-core from frontend's node_modules
const require = createRequire(FRONTEND_DIR + '/package.json');
const chromium = require('playwright-core').chromium;

// --- Screenshot ---

async function captureScreenshot() {
	const browser = await chromium.launch({
		executablePath: chromium.executablePath(),
		headless: true,
		args: BROWSER.launchArgs,
	});

	try {
		const context = await browser.newContext({
			viewport: { width: BROWSER.viewportWidth, height: BROWSER.viewportHeight },
			deviceScaleFactor: BROWSER.deviceScaleFactor,
		});

		const page = await context.newPage();

		page.on('console', msg => {
			if (msg.type() === 'error' || msg.type() === 'warning') {
				console.log(`[browser:${msg.type()}] ${msg.text()}`);
			}
		});

		await page.goto(`${FRONTEND_URL}/game`, { waitUntil: 'networkidle' });

		await page.waitForSelector(SELECTORS.aiVsAiButton, { timeout: SCREENSHOT.uiReadyTimeoutMs });
		await page.click(SELECTORS.aiVsAiButton);
		await page.selectOption('select', timeControl('3+2').value);

		const slider = await page.$(SELECTORS.difficultySlider);
		if (slider) {
			await slider.fill('5');
		}

		await page.click(SELECTORS.newGameButton);

		for (let attempt = 1; attempt <= SCREENSHOT.maxRetries; attempt++) {
			console.log(`Waiting for AI vs AI match to complete (attempt ${attempt}/${SCREENSHOT.maxRetries})...`);

			await page.waitForSelector(SELECTORS.resultBanner, { timeout: SCREENSHOT.bannerWaitMs });

			const bannerText = await page.textContent(SELECTORS.resultBanner);
			if (bannerText && bannerText.includes('Wins!')) {
				console.log(`Game complete: ${bannerText.trim()}`);
				break;
			}

			if (attempt === SCREENSHOT.maxRetries) {
				throw new Error('Failed to get a winning game after max retries');
			}

			console.log('Draw detected, starting new game...');
			await page.click(SELECTORS.newGameButton);
		}

		await page.waitForTimeout(SCREENSHOT.settleMs);

		await page.screenshot({
			path: ARTIFACTS.screenshot,
			fullPage: true,
			type: 'png',
		});

		console.log('Screenshot saved: screenshot.png (repo root)');
	} finally {
		await browser.close();
	}
}

// --- README Update ---

const SCREENSHOT_LINE = '![Caro AI PvP - AI vs AI Match](screenshot.png)';

function updateReadme() {
	const content = readFileSync(ARTIFACTS.readme, 'utf-8');
	const lines = content.split('\n');

	const separatorIdx = lines.findIndex(l => l.startsWith('---'));
	if (separatorIdx === -1) {
		throw new Error('Could not find --- separator in README.md');
	}

	const existingIdx = lines.findIndex(l => l.includes('screenshot.png'));

	if (existingIdx !== -1) {
		lines[existingIdx] = SCREENSHOT_LINE;
	} else {
		lines.splice(separatorIdx, 0, '', SCREENSHOT_LINE, '');
	}

	writeFileSync(ARTIFACTS.readme, lines.join('\n'), 'utf-8');
	console.log('README.md updated');
}

// --- Main ---

async function main() {
	console.log('=== Caro AI PvP - Screenshot Capture ===\n');

	// Step 1-2: Build backend, kill stale processes, start backend
	await startBackend(mgr);

	// Step 3: Start frontend
	console.log('Starting frontend...');
	mgr.spawnDaemon('npm', ['run', 'dev'], FRONTEND_DIR, 'frontend');
	await waitForUrl(FRONTEND_URL, TIMEOUTS.frontendReadyMs);
	console.log('Frontend ready.\n');

	// Step 4: Capture screenshot via real UI
	console.log('\nCapturing screenshot...');
	await captureScreenshot();

	// Step 5: Update README
	updateReadme();

	console.log('\nDone!');
	mgr.cleanup();
	process.exit(0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	mgr.cleanup();
	process.exit(1);
});
