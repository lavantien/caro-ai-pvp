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

import { createWriteStream, writeFileSync } from 'node:fs';
import {
	API_BASE_URL,
	ARTIFACTS,
	DIFFICULTY_NAMES,
	ENDPOINTS,
	GAME_MODE_AIVAI,
	createProcessManager,
	postJson,
	startBackend,
	teeConsole,
	timeControl
} from './lib.mjs';

const logStream = createWriteStream(ARTIFACTS.tournamentLog, { flags: 'w' });
teeConsole(logStream);

const mgr = createProcessManager();

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

// --- Game Logic ---

async function playOneGame(redDiff, blueDiff, timeControlValue, maxMoves, seed) {
	const { gameId } = await postJson(`${API_BASE_URL}${ENDPOINTS.newGame}`, {
		timeControl: timeControlValue,
		gameMode: GAME_MODE_AIVAI,
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
			const data = await postJson(`${API_BASE_URL}${ENDPOINTS.aiMove(gameId)}`);
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
		await fetch(`${API_BASE_URL}${ENDPOINTS.delete(gameId)}`, { method: 'DELETE' }).catch(() => {});
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
	const { games, redDifficulty, blueDifficulty, timeControl: tc, seed, maxMoves, json } = opts;
	timeControl(tc);

	if (!json) {
		console.log('=== Caro AI PvP - Tournament ===');
		console.log(`A: L${redDifficulty} (${DIFFICULTY_NAMES[redDifficulty]})`);
		console.log(`B: L${blueDifficulty} (${DIFFICULTY_NAMES[blueDifficulty]})`);
		console.log(`Games: ${games} | TC: ${tc} | Color swap: every match | Seed: ${seed}`);
		if (games % 2 !== 0) {
			console.log('Warning: an odd game count gives A one extra game with a given color.');
		}
		console.log('');
	}

	// Step 1: Build and start backend
	await startBackend(mgr);

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
			result = await playOneGame(redDiff, blueDiff, tc, maxMoves, seed + i);
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
		config: { games, redDifficulty, blueDifficulty, timeControl: tc, seed, maxMoves },
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
	writeFileSync(ARTIFACTS.tournamentSummary, JSON.stringify(summary, null, 2));

	if (json) {
		console.log(JSON.stringify(summary, null, 2));
	} else {
		console.log('\n=== Summary ===');
		console.log(`A (L${redDifficulty} ${DIFFICULTY_NAMES[redDifficulty]}): ${aWins}/${games}`);
		console.log(`B (L${blueDifficulty} ${DIFFICULTY_NAMES[blueDifficulty]}): ${bWins}/${games}`);
		console.log(`Draws: ${draws} | Errored: ${errored}`);
		console.log(`Red color wins: ${redWins} | Blue color wins: ${blueWins}`);
		console.log(`End reasons: ${JSON.stringify(reasons)}`);
		if (decisive > 0) {
			console.log(`A win rate (decisive games): ${((aWins / decisive) * 100).toFixed(1)}% ` +
				`95% CI [${(ci.low * 100).toFixed(1)}%, ${(ci.high * 100).toFixed(1)}%]`);
		}
		console.log(`Avg moves: ${(totalMoves / Math.max(played.length, 1)).toFixed(1)}`);
		console.log(`Avg time: ${(totalTime / Math.max(played.length, 1)).toFixed(1)}s`);
		console.log('Summary artifact: tournament-summary.json (repo root)');
	}

	mgr.cleanup();
	process.exit(0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	mgr.cleanup();
	process.exit(1);
});
