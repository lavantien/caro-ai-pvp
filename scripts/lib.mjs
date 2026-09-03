/**
 * Shared constants and process helpers for the scripts/ tooling.
 *
 * Single source for URLs/ports (all env-overridable), build paths,
 * timeouts, domain mirrors, artifact paths, UI selectors, and the
 * process-management block the bootstrap scripts previously tripled.
 */

import { spawn, spawnSync } from 'node:child_process';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

// --- URLs and ports ---

// Canonical: Caro.Server Program.cs ServerConfig.DefaultPort; mirrored by
// frontend/src/lib/config ApiConfig.baseUrl. Update together.
export const DEFAULT_API_PORT = 5207;
// Mirrored by frontend/vite.config.ts (commented literal; importing this
// module there breaks svelte-check). Update together.
export const DEFAULT_FRONTEND_PORT = 5173;

export const API_BASE_URL = process.env.API_BASE_URL ?? `http://localhost:${DEFAULT_API_PORT}`;
export const API_PORT = Number(new URL(API_BASE_URL).port) || DEFAULT_API_PORT;
export const WS_UCI_URL =
	process.env.CARO_WS_URL ?? `${API_BASE_URL.replace(/^http/, 'ws')}/ws/uci`;
export const FRONTEND_URL =
	process.env.FRONTEND_URL ?? `http://localhost:${DEFAULT_FRONTEND_PORT}`;

// --- API endpoint paths (mirrors frontend ApiConfig.endpoints) ---

export const ENDPOINTS = Object.freeze({
	newGame: '/api/game/new',
	get: (id) => `/api/game/${id}`,
	aiMove: (id) => `/api/game/${id}/ai-move`,
	delete: (id) => `/api/game/${id}`
});

// --- Build paths ---

export const ROOT = resolve(__dirname, '..');
export const FRONTEND_DIR = resolve(ROOT, 'frontend');
export const SERVER_PROJECT = resolve(ROOT, 'backend', 'src', 'Caro.Server');

/** Mirrors backend/Directory.Build.props. */
export const TFM = process.env.CARO_TFM ?? 'net10.0';

export function serverDll(tfm = TFM, config = 'Debug') {
	return resolve(SERVER_PROJECT, 'bin', config, tfm, 'Caro.Server.dll');
}

// --- Artifact path contracts (filenames also referenced by README/.gitignore) ---

export const ARTIFACTS = Object.freeze({
	screenshot: resolve(ROOT, 'screenshot.png'),
	screenshotVerify: resolve(ROOT, 'screenshot-verify.png'),
	e2eLog: resolve(ROOT, 'e2e.txt'),
	tournamentLog: resolve(ROOT, 'tournament.txt'),
	tournamentSummary: resolve(ROOT, 'tournament-summary.json'),
	readme: resolve(ROOT, 'README.md'),
	positions: resolve(ROOT, 'docs', 'artifacts', 'uci-probes', 'positions.json')
});

// --- Timeouts (env-overridable) ---

function envInt(name, dflt) {
	const v = Number(process.env[name]);
	return Number.isFinite(v) && v > 0 ? v : dflt;
}

export const TIMEOUTS = Object.freeze({
	backendReadyMs: envInt('CARO_BACKEND_TIMEOUT_MS', 60_000),
	frontendReadyMs: envInt('CARO_FRONTEND_TIMEOUT_MS', 30_000),
	// Playwright's budget for booting the dev web server (npm run dev),
	// deliberately larger than frontendReadyMs to cover cold starts.
	webServerTimeoutMs: envInt('CARO_WEBSERVER_TIMEOUT_MS', 120_000),
	defaultReadyMs: 30_000,
	pollIntervalMs: 1_000
});

// --- Domain mirrors (canonical: backend Difficulty.cs / TimeControls.cs, frontend config) ---

export const DIFFICULTY_NAMES = ['', 'Novice', 'Beginner', 'Intermediate', 'Advanced', 'Grandmaster'];
export const GAME_MODE_AIVAI = 'aivai';

export const BOARD = Object.freeze({
	size: 16,
	totalCells: 256,
	columnLabels: 'abcdefghijklmnop'
});

export const TIME_CONTROLS = [
	{ value: '1+0', label: '1+0 Bullet', initialTimeMs: 60_000, incrementSeconds: 0 },
	{ value: '3+0', label: '3+0 Blitz', initialTimeMs: 180_000, incrementSeconds: 0 },
	{ value: '3+2', label: '3+2 Blitz', initialTimeMs: 180_000, incrementSeconds: 2 },
	{ value: '7+5', label: '7+5 Rapid', initialTimeMs: 420_000, incrementSeconds: 5 },
	{ value: '10+0', label: '10+0 Rapid', initialTimeMs: 600_000, incrementSeconds: 0 },
	{ value: '15+10', label: '15+10 Classical', initialTimeMs: 900_000, incrementSeconds: 10 }
];

export const DEFAULT_TIME_CONTROL = '7+5';

// --- Round-robin benchmark defaults (canonical: scripts/run-round-robin.mjs) ---

// Fixed order: 1v1 smoke first (fail fast), then cross pairings strong-vs-weak
// first, 5v5 calibration last. Mirrors the default tournament spec in STATS.md.
export const ROUND_ROBIN_PAIRINGS = Object.freeze([
	[1, 1], [1, 5], [1, 4], [1, 3], [1, 2],
	[2, 5], [2, 4], [2, 3], [3, 5], [3, 4], [4, 5], [5, 5]
]);
export const GAMES_PER_PAIRING_DEFAULT = 20;
export const TOURNAMENT_BASE_SEED = 20260821;
export const ROUND_ROBIN_TIME_CONTROL = '3+2';
// Debug is ~4x slower and gates soft-budget iterations one depth earlier;
// probing compares wall-clock evidence, so the default build is Release.
export const ROUND_ROBIN_BUILD_CONFIG = 'Release';

/** Per-run artifact directory: docs/artifacts/tournaments/<label>/ */
export function tournamentDir(label) {
	return resolve(ROOT, 'docs', 'artifacts', 'tournaments', label);
}

/** One transient-failure-tolerant JSON POST. Throws after `retries` retries. */
export async function postJson(url, body, { retries = 1 } = {}) {
	let lastErr;
	for (let attempt = 0; attempt <= retries; attempt++) {
		try {
			const resp = await fetch(url, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body ?? {})
			});
			if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${(await resp.text()).slice(0, 300)}`);
			return await resp.json();
		} catch (err) {
			lastErr = err;
		}
	}
	throw lastErr;
}

/** Throws on unknown values so UI-option drift fails loudly instead of silently. */
export function timeControl(value) {
	const tc = TIME_CONTROLS.find((t) => t.value === value);
	if (!tc) {
		throw new Error(`Unknown time control "${value}" (expected one of ${TIME_CONTROLS.map((t) => t.value).join(', ')})`);
	}
	return tc;
}

// --- Playwright selectors and browser tunables (shared by capture/verify) ---

export const SELECTORS = Object.freeze({
	aiVsAiButton: 'button:has-text("AI vs AI")',
	newGameButton: 'button:has-text("New Game")',
	resultBanner: '.animate-slide-down',
	difficultySlider: 'input#difficulty',
	moveNotation: '[data-testid="move-notation"]'
});

export const BROWSER = Object.freeze({
	viewportWidth: 1280,
	viewportHeight: 1024,
	deviceScaleFactor: 2,
	launchArgs: ['--disable-gpu', '--no-sandbox']
});

export const SCREENSHOT = Object.freeze({
	maxRetries: envInt('CARO_SCREENSHOT_RETRIES', 3),
	bannerWaitMs: envInt('CARO_BANNER_TIMEOUT_MS', 600_000),
	settleMs: envInt('CARO_SCREENSHOT_SETTLE_MS', 800),
	uiReadyTimeoutMs: 10_000
});

export const PROBE = Object.freeze({
	threads: envInt('CARO_PROBE_THREADS', 1),
	hashMB: envInt('CARO_PROBE_HASH_MB', 256),
	skill: envInt('CARO_PROBE_SKILL', 5),
	speedDepth: envInt('CARO_PROBE_SPEED_DEPTH', 9),
	speedRuns: 3,
	// Large fixed clock so depth is the only binding limit (deterministic).
	clockMs: envInt('CARO_PROBE_CLOCK_MS', 3_600_000),
	connectTimeoutMs: 10_000,
	waitLineTimeoutMs: 120_000,
	parityTimeoutMs: envInt('CARO_PROBE_PARITY_TIMEOUT_MS', 180_000),
	speedTimeoutMs: envInt('CARO_PROBE_SPEED_TIMEOUT_MS', 300_000)
});

// --- Process management ---

export const EXIT_SIGNALS = Object.freeze({ sigint: 130, sigterm: 143 });

const STDERR_CAP = 10_000;
const STDERR_TRIM = 5_000;
const DAEMON_STDERR_CAP = 5_000;
const DAEMON_STDERR_TRIM = 2_500;

/**
 * Registers exit handlers and returns the spawn helpers plus the child
 * list. Each bootstrap script creates exactly one manager.
 */
export function createProcessManager() {
	/** @type {import('node:child_process').ChildProcess[]} */
	const children = [];

	function killPort(port) {
		if (process.platform === 'win32') {
			const r = spawnSync('netstat', ['-ano'], { encoding: 'utf8', shell: false });
			// Lookahead so :5207 does not substring-match :52071.
			const localPort = new RegExp(`:${port}(?=\\s)`);
			for (const line of r.stdout.split('\n')) {
				if (localPort.test(line) && line.includes('LISTENING')) {
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
						spawnSync('taskkill', ['/T', '/F', '/PID', String(child.pid)], { stdio: 'ignore', shell: false });
					} else {
						process.kill(-child.pid);
					}
				}
			} catch { /* already dead */ }
		}
	}

	process.on('exit', cleanup);
	process.on('SIGINT', () => { cleanup(); process.exit(EXIT_SIGNALS.sigint); });
	process.on('SIGTERM', () => { cleanup(); process.exit(EXIT_SIGNALS.sigterm); });

	const needsShell = (cmd) => process.platform === 'win32' && cmd === 'npm';

	function runCommand(command, args, cwd, label) {
		return new Promise((resolvePromise, reject) => {
			const child = spawn(command, args, { cwd, shell: needsShell(command), stdio: ['ignore', 'pipe', 'pipe'] });
			let stderr = '';
			child.stderr?.on('data', (d) => {
				stderr += d.toString();
				if (stderr.length > STDERR_CAP) stderr = stderr.slice(-STDERR_TRIM);
			});
			child.on('error', reject);
			child.on('exit', (code) => {
				if (code === 0) resolvePromise(undefined);
				else reject(new Error(`${label} failed (code ${code}): ${stderr.slice(-500)}`));
			});
		});
	}

	function spawnDaemon(command, args, cwd, label, opts = {}) {
		const child = spawn(command, args, {
			cwd,
			shell: needsShell(command),
			stdio: ['ignore', 'pipe', 'pipe'],
			...opts
		});
		let stderrBuffer = '';
		child.stderr?.on('data', (data) => {
			const text = data.toString();
			stderrBuffer += text;
			if (stderrBuffer.length > DAEMON_STDERR_CAP) stderrBuffer = stderrBuffer.slice(-DAEMON_STDERR_TRIM);
			for (const line of text.split('\n')) {
				if (line.trim()) console.log(`[${label}] ${line}`);
			}
		});
		child.stdout?.on('data', (data) => {
			for (const line of data.toString().split('\n')) {
				if (line.trim()) console.log(`[${label}] ${line}`);
			}
		});
		child.on('error', (err) => console.error(`[${label}] Failed: ${err.message}`));
		child.on('exit', (code) => {
			if (code && code !== 0) {
				console.error(`[${label}] Exited with code ${code}`);
				if (stderrBuffer.trim()) console.error(`[${label}] stderr:\n${stderrBuffer.slice(-1000)}`);
			}
		});
		children.push(child);
		return child;
	}

	return { children, cleanup, killPort, runCommand, spawnDaemon };
}

/**
 * Spawns the backend daemon on API_PORT (via CARO_HTTP_PORT) and resolves
 * when it answers. Kills the port before building so a running server
 * cannot lock its dlls; `env` entries override the defaults (caller wins).
 */
export async function startBackend(mgr, { log = console.log, buildConfig = 'Debug', env = {} } = {}) {
	log(`Killing stale processes on port ${API_PORT}...`);
	mgr.killPort(API_PORT);

	log(`Building backend (${buildConfig})...`);
	await mgr.runCommand('dotnet', ['build', SERVER_PROJECT, '-c', buildConfig], ROOT, 'Build');
	log('Backend built.\n');

	log('Starting backend...');
	mgr.spawnDaemon('dotnet', [serverDll(TFM, buildConfig)], resolve(ROOT, 'backend'), 'backend', {
		env: { ...process.env, CARO_HTTP_PORT: String(API_PORT), ...env }
	});
	await waitForUrl(`${API_BASE_URL}/`, TIMEOUTS.backendReadyMs);
	log('Backend ready.\n');
}

export async function waitForUrl(url, timeoutMs = TIMEOUTS.defaultReadyMs, intervalMs = TIMEOUTS.pollIntervalMs) {
	const start = Date.now();
	while (Date.now() - start < timeoutMs) {
		try {
			const resp = await fetch(url);
			if (resp.ok || resp.status === 404) return;
		} catch { /* not ready */ }
		await new Promise((r) => setTimeout(r, intervalMs));
	}
	throw new Error(`Timeout waiting for ${url} (${timeoutMs}ms)`);
}

/** Tees console output to a log file with timestamps (run-tournament, capture-screenshot). */
export function teeConsole(logStream) {
	const origLog = console.log;
	const origError = console.error;
	const ts = () => new Date().toISOString().slice(11, 23);
	console.log = (...args) => { origLog(...args); logStream.write(`[${ts()}] ${args.join(' ')}\n`); };
	console.error = (...args) => { origError(...args); logStream.write(`[${ts()}] ERR ${args.join(' ')}\n`); };
}
