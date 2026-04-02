#!/usr/bin/env node

/**
 * E2E Screenshot Capture Script
 *
 * Runs the full pipeline (backend + frontend), plays an AI vs AI match,
 * captures a screenshot of the game-winning position, and inserts it into README.md.
 *
 * Usage: node scripts/capture-screenshot.mjs
 */

import { spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');
const FRONTEND_DIR = resolve(ROOT, 'frontend');
const SCREENSHOT_PATH = resolve(ROOT, 'screenshot.png');
const README_PATH = resolve(ROOT, 'README.md');

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';
const FRONTEND_URL = process.env.FRONTEND_URL || 'http://localhost:5173';
const MAX_RETRIES = 3;
const MAX_MOVES = 300;

// Resolve playwright-core from frontend's node_modules
const require = createRequire(resolve(FRONTEND_DIR, 'package.json'));
const pwCore = require('playwright-core');
const chromium = pwCore.chromium;

// --- Process Management ---

/** @type {import('node:child_process').ChildProcess[]} */
const children = [];

function cleanup() {
	for (const child of children) {
		try {
			if (child.pid) {
				if (process.platform === 'win32') {
					// Tree kill on Windows to handle grandchild processes
					spawn('taskkill', ['/T', '/F', '/PID', String(child.pid)], {
						stdio: 'ignore',
						shell: true,
					});
				} else {
					process.kill(-child.pid); // kill process group
				}
			}
		} catch { /* already dead */ }
	}
}

process.on('exit', cleanup);
process.on('SIGINT', () => { cleanup(); process.exit(130); });
process.on('SIGTERM', () => { cleanup(); process.exit(143); });

/**
 * Run a command and wait for it to exit. Throws on non-zero exit.
 * @param {string} command
 * @param {string[]} args
 * @param {string} cwd
 * @param {string} label
 */
function runCommand(command, args, cwd, label) {
	return new Promise((resolve, reject) => {
		const child = spawn(command, args, {
			cwd,
			shell: true,
			stdio: ['ignore', 'pipe', 'pipe'],
		});

		let stderr = '';
		child.stderr?.on('data', (d) => {
			stderr += d.toString();
			if (stderr.length > 10000) stderr = stderr.slice(-5000);
		});

		child.on('error', reject);
		child.on('exit', (code) => {
			if (code === 0) resolve(undefined);
			else reject(new Error(`${label} failed (code ${code}): ${stderr.slice(-500)}`));
		});
	});
}

/**
 * Spawn a long-lived child process (daemon-like).
 * @param {string} command
 * @param {string[]} args
 * @param {string} cwd
 * @param {string} label
 */
function spawnDaemon(command, args, cwd, label) {
	const child = spawn(command, args, {
		cwd,
		shell: true,
		stdio: ['ignore', 'pipe', 'pipe'],
	});

	let stderrBuffer = '';
	child.stderr?.on('data', (data) => {
		stderrBuffer += data.toString();
		if (stderrBuffer.length > 5000) stderrBuffer = stderrBuffer.slice(-2500);
	});

	child.on('error', (err) => console.error(`[${label}] Failed: ${err.message}`));
	child.on('exit', (code) => {
		if (code && code !== 0) {
			console.error(`[${label}] Exited with code ${code}`);
			if (stderrBuffer.trim()) {
				console.error(`[${label}] stderr:\n${stderrBuffer.slice(-1000)}`);
			}
		}
	});

	children.push(child);
	return child;
}

// --- Health Checks ---

async function waitForUrl(url, timeoutMs = 30_000, intervalMs = 1000) {
	const start = Date.now();
	while (Date.now() - start < timeoutMs) {
		try {
			const resp = await fetch(url);
			if (resp.ok || resp.status === 404) return;
		} catch { /* not ready */ }
		await new Promise(r => setTimeout(r, intervalMs));
	}
	throw new Error(`Timeout waiting for ${url} (${timeoutMs}ms)`);
}

// --- Game Logic ---

async function playGame() {
	const createResp = await fetch(`${API_BASE}/api/game/new`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ timeControl: '7+5', gameMode: 'aivai' }),
	});
	if (!createResp.ok) throw new Error(`Create game failed: ${createResp.status}`);
	const { gameId, state: initialState } = await createResp.json();

	/** @type {{ moveNumber: number; player: string; x: number; y: number }[]} */
	const moveHistory = [];
	let currentState = initialState;
	let prevBoard = initialState.board;

	console.log(`Game created: ${gameId}`);

	for (let i = 0; i < MAX_MOVES; i++) {
		const moveResp = await fetch(`${API_BASE}/api/game/${gameId}/ai-move`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({}),
		});

		if (!moveResp.ok) {
			console.error(`AI move failed: ${moveResp.status} - ${await moveResp.text()}`);
			break;
		}

		const { state } = await moveResp.json();
		currentState = state;

		const newMove = findNewMove(prevBoard, state.board);
		if (newMove) {
			moveHistory.push({
				moveNumber: state.moveNumber,
				player: state.currentPlayer === 'red' ? 'blue' : 'red',
				x: newMove.x,
				y: newMove.y,
			});
		}

		prevBoard = state.board;

		if (state.isGameOver) {
			console.log(`Game over after ${moveHistory.length} moves. Winner: ${state.winner}`);
			break;
		}
	}

	if (!currentState.isGameOver) {
		throw new Error('Game did not finish within move limit');
	}

	return { gameId, state: currentState, moveHistory };
}

function findNewMove(oldBoard, newBoard) {
	for (let i = 0; i < oldBoard.length; i++) {
		if (oldBoard[i].player === 'none' && newBoard[i].player !== 'none') {
			return { x: newBoard[i].x, y: newBoard[i].y };
		}
	}
	return null;
}

// --- Screenshot ---

async function captureScreenshot(gameState, moveHistory) {
	const browser = await chromium.launch({
		executablePath: chromium.executablePath(),
		headless: true,
		args: ['--disable-gpu', '--no-sandbox'],
	});

	try {
		const context = await browser.newContext({
			viewport: { width: 1280, height: 1024 },
			deviceScaleFactor: 2,
		});

		const page = await context.newPage();

		// Intercept API calls to inject our completed game state
		await page.route('**/api/game/new', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ gameId: 'screenshot-capture', state: gameState }),
			});
		});

		await page.route('**/api/game/screenshot-capture', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ state: gameState }),
			});
		});

		// Block AI and regular move attempts
		await page.route('**/api/game/*/ai-move', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ state: gameState }),
			});
		});

		await page.route('**/api/game/*/move', async (route) => {
			await route.fulfill({
				status: 400,
				contentType: 'text/plain',
				body: 'Game is over',
			});
		});

		await page.goto(`${FRONTEND_URL}/game`, { waitUntil: 'networkidle' });
		await page.waitForSelector('.grid.gap-0', { timeout: 10_000 });

		// Inject move history (frontend builds it incrementally, not from API state)
		await page.evaluate((moves) => {
			// Find the Move History section by its heading (more reliable than class selector)
			const headings = document.querySelectorAll('h3');
			let historySection = null;
			for (const h of headings) {
				if (h.textContent?.includes('Move History')) {
					historySection = h.parentElement;
					break;
				}
			}
			if (!historySection) return;

			// Remove "No moves yet" placeholder and any existing list
			const noMovesText = historySection.querySelector('p.text-gray-500');
			if (noMovesText) noMovesText.remove();
			const existingList = historySection.querySelector('.overflow-y-auto.max-h-64');
			if (existingList) existingList.remove();

			// Build move list
			const container = document.createElement('div');
			container.className = 'border rounded-lg p-4 bg-white shadow-sm overflow-y-auto max-h-64';
			for (const move of moves) {
				const row = document.createElement('div');
				const isLatest = move.moveNumber === moves.length;
				const bgClass = isLatest
					? (move.player === 'red' ? 'bg-red-100' : 'bg-blue-100')
					: '';
				row.className = `flex justify-between items-center py-1 px-2 rounded ${bgClass}`;
				const label = `${move.moveNumber}. ${move.player.charAt(0).toUpperCase() + move.player.slice(1)}: (${move.x}, ${move.y})`;
				row.innerHTML = `<span class="text-sm font-medium text-gray-700">${label}</span>`;
				container.appendChild(row);
			}
			historySection.appendChild(container);
		}, moveHistory);

		// Wait for winning line animation
		await page.waitForTimeout(800);

		await page.screenshot({
			path: SCREENSHOT_PATH,
			fullPage: true,
			type: 'png',
		});

		console.log(`Screenshot saved: ${SCREENSHOT_PATH}`);
	} finally {
		await browser.close();
	}
}

// --- README Update ---

const SCREENSHOT_LINE = '![Caro AI PvP - AI vs AI Match](screenshot.png)';

function updateReadme() {
	const content = readFileSync(README_PATH, 'utf-8');
	const lines = content.split('\n');

	// Find the first --- separator
	const separatorIdx = lines.findIndex(l => l.startsWith('---'));
	if (separatorIdx === -1) {
		throw new Error('Could not find --- separator in README.md');
	}

	// Check if screenshot line already exists
	const existingIdx = lines.findIndex(l => l.includes('screenshot.png'));

	if (existingIdx !== -1) {
		lines[existingIdx] = SCREENSHOT_LINE;
	} else {
		// Insert blank line + screenshot + blank line before ---
		lines.splice(separatorIdx, 0, '', SCREENSHOT_LINE, '');
	}

	writeFileSync(README_PATH, lines.join('\n'), 'utf-8');
	console.log('README.md updated');
}

// --- Main ---

async function main() {
	console.log('=== Caro AI PvP - Screenshot Capture ===\n');

	// Step 1: Build backend
	console.log('Building backend...');
	await runCommand('dotnet', ['build', 'backend/src/Caro.Api'], ROOT, 'Build');
	console.log('Backend built.\n');

	// Step 2: Start backend
	console.log('Starting backend...');
	spawnDaemon(
		'dotnet', ['run', '--project', 'backend/src/Caro.Api', '--no-build'],
		ROOT,
		'backend',
	);
	await waitForUrl(`${API_BASE}/`, 60_000);
	console.log('Backend ready.\n');

	// Step 3: Start frontend
	console.log('Starting frontend...');
	spawnDaemon('npm', ['run', 'dev'], FRONTEND_DIR, 'frontend');
	await waitForUrl(FRONTEND_URL, 30_000);
	console.log('Frontend ready.\n');

	// Step 4: Play AI vs AI match (retry on draw)
	let gameResult;
	for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
		console.log(`Playing AI vs AI match (attempt ${attempt}/${MAX_RETRIES})...`);
		try {
			gameResult = await playGame();
			if (gameResult.state.winner && gameResult.state.winner !== 'none') {
				break;
			}
			console.log('No winner, retrying...');
		} catch (err) {
			console.error(`Attempt ${attempt} failed: ${err.message}`);
			if (attempt === MAX_RETRIES) throw err;
		}
	}

	if (!gameResult) {
		throw new Error('Failed to complete a game');
	}

	// Step 5: Capture screenshot
	console.log('\nCapturing screenshot...');
	await captureScreenshot(gameResult.state, gameResult.moveHistory);

	// Step 6: Update README
	updateReadme();

	console.log('\nDone!');
	cleanup();
	process.exit(0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	cleanup();
	process.exit(1);
});
