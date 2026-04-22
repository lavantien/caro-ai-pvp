#!/usr/bin/env node

/**
 * Self-contained AI tournament: builds/starts backend, runs N matches with
 * color swapping, and reports aggregate results.
 *
 * Usage: node scripts/run-tournament.mjs [--games N] [--red N] [--blue N] [--tc TIME] [--json]
 *
 * Examples:
 *   node scripts/run-tournament.mjs --games 10 --red 1 --blue 5 --tc 3+2
 *   node scripts/run-tournament.mjs --games 4 --red 3 --blue 4 --tc 7+5
 */

import { spawn, spawnSync } from 'node:child_process';
import { createWriteStream } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');

const LOG_PATH = resolve(ROOT, 'tournament.txt');
const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';
const NAMES = ['', 'Novice', 'Beginner', 'Intermediate', 'Advanced', 'Grandmaster'];

// --- Logging ---

const logStream = createWriteStream(LOG_PATH, { flags: 'w' });
const origLog = console.log;
const origError = console.error;
function ts() { return new Date().toISOString().slice(11, 23); }
console.log = (...args) => { origLog(...args); logStream.write(`[${ts()}] ${args.join(' ')}\n`); };
console.error = (...args) => { origError(...args); logStream.write(`[${ts()}] ERR ${args.join(' ')}\n`); };

// --- CLI ---

function parseArgs() {
	const args = process.argv.slice(2);
	const opts = { games: 10, redDifficulty: 1, blueDifficulty: 5, timeControl: '3+2', maxMoves: 200, json: false };
	for (let i = 0; i < args.length; i++) {
		switch (args[i]) {
			case '--games': opts.games = parseInt(args[++i], 10); break;
			case '--red': opts.redDifficulty = parseInt(args[++i], 10); break;
			case '--blue': opts.blueDifficulty = parseInt(args[++i], 10); break;
			case '--tc': opts.timeControl = args[++i]; break;
			case '--max-moves': opts.maxMoves = parseInt(args[++i], 10); break;
			case '--json': opts.json = true; break;
			case '--help':
				console.log('Usage: node scripts/run-tournament.mjs [options]');
				console.log('');
				console.log('Options:');
				console.log('  --games N      Number of matches (default 10)');
				console.log('  --red N        Red player difficulty 1-5 (default 1)');
				console.log('  --blue N       Blue player difficulty 1-5 (default 5)');
				console.log('  --tc TIME      Time control (default 3+2)');
				console.log('  --max-moves N  Max moves before draw (default 200)');
				console.log('  --json         Output results as JSON');
				process.exit(0);
		}
	}
	return opts;
}

// --- Process Management ---

const children = [];

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

function runCommand(command, args, cwd, label) {
	return new Promise((resolve, reject) => {
		const child = spawn(command, args, { cwd, shell: true, stdio: ['ignore', 'pipe', 'pipe'] });
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
	const child = spawn(command, args, { cwd, shell: true, stdio: ['ignore', 'pipe', 'pipe'] });
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

// --- Game Logic ---

async function playOneGame(redDiff, blueDiff, timeControl, maxMoves) {
	const createResp = await fetch(`${API_BASE}/api/game/new`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			timeControl,
			gameMode: 'aivai',
			redDifficulty: redDiff,
			blueDifficulty: blueDiff,
		}),
	});
	if (!createResp.ok) throw new Error(`Create game failed: ${await createResp.text()}`);
	const { gameId } = await createResp.json();

	const startTime = Date.now();
	let moveCount = 0;
	let winner = null;
	let reason = '';

	while (moveCount < maxMoves) {
		const moveResp = await fetch(`${API_BASE}/api/game/${gameId}/ai-move`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: '{}',
		});
		if (!moveResp.ok) {
			throw new Error(`AI move failed at move ${moveCount + 1} (HTTP ${moveResp.status}): ${await moveResp.text()}`);
		}
		const data = await moveResp.json();
		moveCount++;

		if (data.state.isGameOver) {
			winner = data.state.winner;
			reason = winner ? 'win' : 'draw';
			break;
		}
	}

	if (!winner && moveCount >= maxMoves) reason = 'max-moves';
	const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

	// Delete game from server to free AI engine memory
	await fetch(`${API_BASE}/api/game/${gameId}`, { method: 'DELETE' }).catch(() => {});

	return { gameId, redDiff, blueDiff, moves: moveCount, winner: winner || 'none', reason, elapsedSeconds: parseFloat(elapsed) };
}

// --- Main ---

async function main() {
	const opts = parseArgs();
	const { games, redDifficulty, blueDifficulty, timeControl, maxMoves, json } = opts;

	if (!json) {
		console.log('=== Caro AI PvP - Tournament ===');
		console.log(`A: L${redDifficulty} (${NAMES[redDifficulty]})`);
		console.log(`B: L${blueDifficulty} (${NAMES[blueDifficulty]})`);
		console.log(`Games: ${games} | TC: ${timeControl} | Color swap: every match`);
		console.log('');
	}

	// Step 1: Build and start backend
	console.log('Building backend...');
	await runCommand('dotnet', ['build', 'backend/src/Caro.Api'], ROOT, 'Build');
	console.log('Backend built.');

	console.log('Starting backend...');
	spawnDaemon('dotnet', ['run', '--project', 'backend/src/Caro.Api', '--no-build'], ROOT, 'backend');
	await waitForUrl(`${API_BASE}/`, 60_000);
	console.log('Backend ready.\n');

	// Step 2: Run matches with color swapping
	const results = [];

	for (let i = 1; i <= games; i++) {
		const swap = i % 2 === 0;
		const redDiff = swap ? blueDifficulty : redDifficulty;
		const blueDiff = swap ? redDifficulty : blueDifficulty;
		const redLabel = swap ? 'B' : 'A';
		const blueLabel = swap ? 'A' : 'B';

		console.log(`Match ${i}/${games}: Red=L${redDiff}(${redLabel}) Blue=L${blueDiff}(${blueLabel})${swap ? ' (swapped)' : ''}`);

		const result = await playOneGame(redDiff, blueDiff, timeControl, maxMoves);
		results.push(result);

		const winnerColor = result.winner === 'none' ? 'DRAW' : result.winner.toUpperCase();
		const levelLabel = result.winner === 'red'
			? `L${redDiff}(${redLabel})`
			: result.winner === 'blue'
				? `L${blueDiff}(${blueLabel})`
				: 'none';

		if (!json) {
			console.log(`  -> ${winnerColor} wins by ${result.reason} | ${result.moves} moves | ${result.elapsedSeconds}s | Winner level: ${levelLabel}`);
		}
	}

	// Step 3: Summary
	const aWins = results.filter(r => {
		const swap = results.indexOf(r) % 2 === 1;
		// swap=false: A=red, swap=true: A=blue
		if (!swap) return r.winner === 'red';
		return r.winner === 'blue';
	}).length;

	const bWins = results.filter(r => {
		const swap = results.indexOf(r) % 2 === 1;
		if (!swap) return r.winner === 'blue';
		return r.winner === 'red';
	}).length;

	const draws = results.filter(r => r.winner === 'none').length;
	const totalMoves = results.reduce((s, r) => s + r.moves, 0);
	const totalTime = results.reduce((s, r) => s + r.elapsedSeconds, 0);

	if (json) {
		console.log(JSON.stringify({
			config: { games, redDifficulty, blueDifficulty, timeControl, maxMoves },
			results,
			summary: {
				aWins,
				bWins,
				draws,
				aWinRate: `${((aWins / games) * 100).toFixed(1)}%`,
				bWinRate: `${((bWins / games) * 100).toFixed(1)}%`,
				avgMoves: parseFloat((totalMoves / games).toFixed(1)),
				avgTime: parseFloat((totalTime / games).toFixed(1)),
			},
		}, null, 2));
	} else {
		console.log('\n=== Summary ===');
		console.log(`A (L${redDifficulty} ${NAMES[redDifficulty]}): ${aWins}/${games} (${((aWins / games) * 100).toFixed(1)}%)`);
		console.log(`B (L${blueDifficulty} ${NAMES[blueDifficulty]}): ${bWins}/${games} (${((bWins / games) * 100).toFixed(1)}%)`);
		console.log(`Draws: ${draws}`);
		console.log(`Avg moves: ${(totalMoves / games).toFixed(1)}`);
		console.log(`Avg time: ${(totalTime / games).toFixed(1)}s`);
	}

	cleanup();
	process.exit(0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	cleanup();
	process.exit(1);
});
