#!/usr/bin/env node

/**
 * Self-contained AI tournament: builds/starts backend, runs N matches with
 * color swapping and seeded opening randomization, and reports aggregate
 * results with a 95% Wilson score interval.
 *
 * Usage: node scripts/run-tournament.mjs [--games N] [--red N] [--blue N]
 *        [--tc TIME] [--seed N] [--max-moves N] [--json]
 *
 * Examples:
 *   node scripts/run-tournament.mjs --games 10 --red 1 --blue 5 --tc 3+2
 *   node scripts/run-tournament.mjs --games 4 --red 3 --blue 4 --tc 7+5
 */

import { spawn, spawnSync } from 'node:child_process';
import { createWriteStream, writeFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');

const LOG_PATH = resolve(ROOT, 'tournament.txt');
const SUMMARY_PATH = resolve(ROOT, 'tournament-summary.json');
const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';
const NAMES = ['', 'Novice', 'Beginner', 'Intermediate', 'Advanced', 'Grandmaster'];

// --- Logging ---

const logStream = createWriteStream(LOG_PATH, { flags: 'w' });
const origLog = console.log;
const origError = console.error;
function ts() { return new Date().toISOString().slice(11, 23); }
console.log = (...args) => { origLog(...args); logStream.write(`[${ts()}] ${args.join(' ')}\n`); };
console.error = (...args) => { origLog(...args); logStream.write(`[${ts()}] ERR ${args.join(' ')}\n`); };

// --- CLI ---

function parseArgs() {
	const args = process.argv.slice(2);
	const opts = { games: 10, redDifficulty: 1, blueDifficulty: 5, timeControl: '3+2', seed: 20260821, maxMoves: 200, json: false };
	for (let i = 0; i < args.length; i++) {
		switch (args[i]) {
			case '--games': opts.games = parseInt(args[++i], 10); break;
			case '--red': opts.redDifficulty = parseInt(args[++i], 10); break;
			case '--blue': opts.blueDifficulty = parseInt(args[++i], 10); break;
			case '--tc': opts.timeControl = args[++i]; break;
			case '--seed': opts.seed = parseInt(args[++i], 10); break;
			case '--max-moves': opts.maxMoves = parseInt(args[++i], 10); break;
			case '--json': opts.json = true; break;
			case '--help':
				console.log('Usage: node scripts/run-tournament.mjs [options]');
				console.log('');
				console.log('Options:');
				console.log('  --games N      Number of matches (default 10)');
				console.log('  --red N        Player A difficulty 1-5 (default 1)');
				console.log('  --blue N       Player B difficulty 1-5 (default 5)');
				console.log('  --tc TIME      Time control (default 3+2)');
				console.log('  --seed N       Opening randomization seed (default 20260821)');
				console.log('  --max-moves N  Max moves before draw (default 200)');
				console.log('  --json         Output results as JSON');
				process.exit(0);
		}
	}
	return opts;
}

/** 95% Wilson score interval for a proportion. */
function wilsonInterval(wins, n) {
	if (n === 0) return { low: 0, high: 1 };
	const z = 1.959963984540054; // two-sided 95%
	const p = wins / n;
	const denom = 1 + (z * z) / n;
	const center = (p + (z * z) / (2 * n)) / denom;
	const half = (z * Math.sqrt((p * (1 - p)) / n + (z * z) / (4 * n * n))) / denom;
	return { low: Math.max(0, center - half), high: Math.min(1, center + half) };
}

// --- Process Management ---

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
					spawnSync('taskkill', ['/T', '/F', '/PID', String(child.pid)], { stdio: 'ignore', shell: false });
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
		const child = spawn(command, args, { cwd, shell: needsShell(command), stdio: ['ignore', 'pipe', 'pipe'] });
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
	const child = spawn(command, args, { cwd, shell: needsShell(command), stdio: ['ignore', 'pipe', 'pipe'] });
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

/** One transient-failure-tolerant JSON POST. Throws after one retry. */
async function postJson(url, body) {
	let lastErr;
	for (let attempt = 0; attempt < 2; attempt++) {
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

// --- Game Logic ---

async function playOneGame(redDiff, blueDiff, timeControl, maxMoves, seed) {
	const { gameId } = await postJson(`${API_BASE}/api/game/new`, {
		timeControl,
		gameMode: 'aivai',
		redDifficulty: redDiff,
		blueDifficulty: blueDiff,
		randomOpening: true,
		seed
	});

	const startTime = Date.now();
	let moveCount = 0;
	let winner = null;
	let reason = '';

	try {
		while (moveCount < maxMoves) {
			const data = await postJson(`${API_BASE}/api/game/${gameId}/ai-move`);
			moveCount++;

			if (data.lastMove?.statline) console.log(data.lastMove.statline);

			if (data.state.isGameOver) {
				winner = data.state.winner || 'none';
				reason = data.state.endReason || (winner === 'none' ? 'draw' : 'win');
				break;
			}
		}
	} finally {
		// Free engine memory regardless of outcome.
		await fetch(`${API_BASE}/api/game/${gameId}`, { method: 'DELETE' }).catch(() => {});
	}

	if (!winner && moveCount >= maxMoves) {
		winner = 'none';
		reason = 'max-moves';
	}
	const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

	return { gameId, redDiff, blueDiff, moves: moveCount, winner, reason, elapsedSeconds: parseFloat(elapsed) };
}

// --- Main ---

async function main() {
	const opts = parseArgs();
	const { games, redDifficulty, blueDifficulty, timeControl, seed, maxMoves, json } = opts;

	if (!json) {
		console.log('=== Caro AI PvP - Tournament ===');
		console.log(`A: L${redDifficulty} (${NAMES[redDifficulty]})`);
		console.log(`B: L${blueDifficulty} (${NAMES[blueDifficulty]})`);
		console.log(`Games: ${games} | TC: ${timeControl} | Color swap: every match | Seed: ${seed}`);
		if (games % 2 !== 0) {
			console.log('Warning: an odd game count gives A one extra game with a given color.');
		}
		console.log('');
	}

	// Step 1: Build and start backend
	const serverProject = resolve(ROOT, 'backend', 'src', 'Caro.Server');
	const serverDll = resolve(serverProject, 'bin', 'Debug', 'net10.0', 'Caro.Server.dll');

	console.log('Building backend...');
	await runCommand('dotnet', ['build', serverProject, '-c', 'Debug'], ROOT, 'Build');
	console.log('Backend built.');

	console.log('Killing stale processes on port 5207...');
	killPort(5207);

	console.log('Starting backend...');
	spawnDaemon('dotnet', [serverDll], resolve(ROOT, 'backend'), 'backend');
	await waitForUrl(`${API_BASE}/`, 60_000);
	console.log('Backend ready.\n');

	// Step 2: Run matches with color swapping and per-game opening seeds
	const results = [];

	for (let i = 1; i <= games; i++) {
		const swap = i % 2 === 0;
		const redDiff = swap ? blueDifficulty : redDifficulty;
		const blueDiff = swap ? redDifficulty : blueDifficulty;
		const redLabel = swap ? 'B' : 'A';
		const blueLabel = swap ? 'A' : 'B';

		console.log(`Match ${i}/${games}: Red=L${redDiff}(${redLabel}) Blue=L${blueDiff}(${blueLabel})${swap ? ' (swapped)' : ''} seed=${seed + i}`);

		let result;
		try {
			result = await playOneGame(redDiff, blueDiff, timeControl, maxMoves, seed + i);
			result.errored = false;
		} catch (err) {
			console.error(`  -> ERRORED: ${err.message}`);
			result = { redDiff, blueDiff, moves: 0, winner: 'none', reason: 'errored', elapsedSeconds: 0, errored: true };
		}
		result.swap = swap;
		result.index = i;
		results.push(result);

		if (!result.errored && !json) {
			const winnerColor = result.winner === 'none' ? 'DRAW' : result.winner.toUpperCase();
			const levelLabel = result.winner === 'red'
				? `L${redDiff}(${redLabel})`
				: result.winner === 'blue'
					? `L${blueDiff}(${blueLabel})`
					: 'none';
			console.log(`  -> ${winnerColor} by ${result.reason} | ${result.moves} moves | ${result.elapsedSeconds}s | Winner level: ${levelLabel}`);
		}
	}

	// Step 3: Summary with per-color, per-reason, and interval statistics
	const played = results.filter(r => !r.errored);
	const aWins = played.filter(r => (r.winner === 'red') !== r.swap).length;
	const bWins = played.filter(r => (r.winner === 'blue') !== r.swap).length;
	const draws = played.filter(r => r.winner === 'none').length;
	const errored = results.length - played.length;

	const redWins = played.filter(r => r.winner === 'red').length;
	const blueWins = played.filter(r => r.winner === 'blue').length;
	const reasons = {};
	for (const r of played) reasons[r.reason] = (reasons[r.reason] || 0) + 1;

	const totalMoves = played.reduce((s, r) => s + r.moves, 0);
	const totalTime = played.reduce((s, r) => s + r.elapsedSeconds, 0);
	const decisive = aWins + bWins;
	const ci = wilsonInterval(aWins, decisive);

	const summary = {
		config: { games, redDifficulty, blueDifficulty, timeControl, seed, maxMoves },
		summary: {
			aWins, bWins, draws, errored,
			redWins, blueWins,
			reasons,
			aWinRateDecisive: decisive > 0 ? aWins / decisive : null,
			aWinRate95CI: decisive > 0 ? { low: ci.low, high: ci.high } : null,
			avgMoves: played.length ? totalMoves / played.length : 0,
			avgTime: played.length ? totalTime / played.length : 0
		},
		results
	};
	writeFileSync(SUMMARY_PATH, JSON.stringify(summary, null, 2));

	if (json) {
		console.log(JSON.stringify(summary, null, 2));
	} else {
		console.log('\n=== Summary ===');
		console.log(`A (L${redDifficulty} ${NAMES[redDifficulty]}): ${aWins}/${games}`);
		console.log(`B (L${blueDifficulty} ${NAMES[blueDifficulty]}): ${bWins}/${games}`);
		console.log(`Draws: ${draws} | Errored: ${errored}`);
		console.log(`Red color wins: ${redWins} | Blue color wins: ${blueWins}`);
		console.log(`End reasons: ${JSON.stringify(reasons)}`);
		if (decisive > 0) {
			console.log(`A win rate (decisive games): ${((aWins / decisive) * 100).toFixed(1)}% ` +
				`95% CI [${(ci.low * 100).toFixed(1)}%, ${(ci.high * 100).toFixed(1)}%]`);
		}
		console.log(`Avg moves: ${(totalMoves / Math.max(played.length, 1)).toFixed(1)}`);
		console.log(`Avg time: ${(totalTime / Math.max(played.length, 1)).toFixed(1)}s`);
		console.log(`Summary artifact: ${SUMMARY_PATH}`);
	}

	cleanup();
	process.exit(0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	cleanup();
	process.exit(1);
});
