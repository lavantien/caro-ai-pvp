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

import { spawn, spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { readFileSync, writeFileSync, createWriteStream } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');
const FRONTEND_DIR = resolve(ROOT, 'frontend');
const SCREENSHOT_PATH = resolve(ROOT, 'screenshot.png');
const README_PATH = resolve(ROOT, 'README.md');
const LOG_PATH = resolve(ROOT, 'e2e.txt');

// Tee all output to e2e.txt
const logStream = createWriteStream(LOG_PATH, { flags: 'w' });
const origLog = console.log;
const origError = console.error;
function ts() { return new Date().toISOString().slice(11, 23); }
console.log = (...args) => { origLog(...args); logStream.write(`[${ts()}] ${args.join(' ')}\n`); };
console.error = (...args) => { origError(...args); logStream.write(`[${ts()}] ERR ${args.join(' ')}\n`); };

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';
const FRONTEND_URL = process.env.FRONTEND_URL || 'http://localhost:5173';
const MAX_RETRIES = 3;

// Resolve playwright-core from frontend's node_modules
const require = createRequire(resolve(FRONTEND_DIR, 'package.json'));
const pwCore = require('playwright-core');
const chromium = pwCore.chromium;

// --- Process Management ---

/** @type {import('node:child_process').ChildProcess[]} */
const children = [];

function killPort(port) {
	if (process.platform === 'win32') {
		const r = spawnSync('netstat', ['-ano'], { encoding: 'utf8', shell: false });
		for (const line of r.stdout.split('\n')) {
			if (line.includes(`:${port}`) && line.includes('LISTENING')) {
				const pid = line.trim().split(/\s+/).pop();
				if (pid && /^\d+$/.test(pid)) {
					spawnSync('taskkill', ['/F', '/PID', pid], { stdio: 'ignore', shell: false });
				}
			}
		}
	} else {
		spawnSync('sh', ['-c', `lsof -ti:${port} | xargs kill -9 2>/dev/null || true`], { stdio: 'ignore' });
	}
}

function cleanup() {
	for (const child of children) {
		try {
			if (child.pid) {
				if (process.platform === 'win32') {
					spawnSync('taskkill', ['/T', '/F', '/PID', String(child.pid)], {
						stdio: 'ignore',
						shell: false,
					});
				} else {
					process.kill(-child.pid);
				}
			}
		} catch { /* already dead */ }
	}
}

process.on('exit', cleanup);
process.on('SIGINT', () => { cleanup(); process.exit(130); });
process.on('SIGTERM', () => { cleanup(); process.exit(143); });

const needsShell = (cmd) => process.platform === 'win32' && cmd === 'npm';

function runCommand(command, args, cwd, label) {
	return new Promise((resolve, reject) => {
		const child = spawn(command, args, {
			cwd,
			shell: needsShell(command),
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

function spawnDaemon(command, args, cwd, label) {
	const child = spawn(command, args, {
		cwd,
		shell: needsShell(command),
		stdio: ['ignore', 'pipe', 'pipe'],
	});

	let stderrBuffer = '';
	child.stderr?.on('data', (data) => {
		const text = data.toString();
		stderrBuffer += text;
		if (stderrBuffer.length > 5000) stderrBuffer = stderrBuffer.slice(-2500);
		for (const line of text.split('\n')) {
			if (line.trim()) console.log(`[${label}] ${line}`);
		}
	});

	child.stdout?.on('data', (data) => {
		const text = data.toString();
		for (const line of text.split('\n')) {
			if (line.trim()) console.log(`[${label}] ${line}`);
		}
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

// --- Screenshot ---

async function captureScreenshot() {
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

		page.on('console', msg => {
			if (msg.type() === 'error' || msg.type() === 'warning') {
				console.log(`[browser:${msg.type()}] ${msg.text()}`);
			}
		});

		await page.goto(`${FRONTEND_URL}/game`, { waitUntil: 'networkidle' });

		await page.waitForSelector('button:has-text("AI vs AI")', { timeout: 10_000 });
		await page.click('button:has-text("AI vs AI")');
		await page.selectOption('select', '3+2');

		const slider = await page.$('input#difficulty');
		if (slider) {
			await slider.fill('5');
		}

		await page.click('button:has-text("New Game")');

		for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
			console.log(`Waiting for AI vs AI match to complete (attempt ${attempt}/${MAX_RETRIES})...`);

			await page.waitForSelector('.animate-slide-down', { timeout: 600_000 });

			const bannerText = await page.textContent('.animate-slide-down');
			if (bannerText && bannerText.includes('Wins!')) {
				console.log(`Game complete: ${bannerText.trim()}`);
				break;
			}

			if (attempt === MAX_RETRIES) {
				throw new Error('Failed to get a winning game after max retries');
			}

			console.log('Draw detected, starting new game...');
			await page.click('button:has-text("New Game")');
		}

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

	writeFileSync(README_PATH, lines.join('\n'), 'utf-8');
	console.log('README.md updated');
}

// --- Main ---

async function main() {
	console.log('=== Caro AI PvP - Screenshot Capture ===\n');

	const serverProject = resolve(ROOT, 'backend', 'src', 'Caro.Server');
	const serverDll = resolve(serverProject, 'bin', 'Debug', 'net10.0', 'Caro.Server.dll');

	// Step 1: Build backend
	console.log('Building backend...');
	await runCommand('dotnet', ['build', serverProject, '-c', 'Debug'], ROOT, 'Build');
	console.log('Backend built.\n');

	// Step 2: Kill stale processes and start backend
	console.log('Killing stale processes on port 5207...');
	killPort(5207);

	console.log('Starting backend...');
	spawnDaemon('dotnet', [serverDll], resolve(ROOT, 'backend'), 'backend');
	await waitForUrl(`${API_BASE}/`, 60_000);
	console.log('Backend ready.\n');

	// Step 3: Start frontend
	console.log('Starting frontend...');
	spawnDaemon('npm', ['run', 'dev'], FRONTEND_DIR, 'frontend');
	await waitForUrl(FRONTEND_URL, 30_000);
	console.log('Frontend ready.\n');

	// Step 4: Capture screenshot via real UI
	console.log('\nCapturing screenshot...');
	await captureScreenshot();

	// Step 5: Update README
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
