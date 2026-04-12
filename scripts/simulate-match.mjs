#!/usr/bin/env node

/**
 * Simulate an AI vs AI match through the HTTP API with per-player difficulty.
 * Usage: node scripts/simulate-match.mjs [--red N] [--blue N] [--tc 3+2] [--max-moves 200] [--json]
 *
 * Examples:
 *   node scripts/simulate-match.mjs --red 5 --blue 1     # Grandmaster vs Novice
 *   node scripts/simulate-match.mjs --red 4 --blue 4     # Advanced vs Advanced
 *   node scripts/simulate-match.mjs --red 5 --blue 3     # GM vs Intermediate
 */

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5207';

function parseArgs() {
	const args = process.argv.slice(2);
	const opts = { redDifficulty: 5, blueDifficulty: 3, timeControl: '7+5', maxMoves: 200, json: false };
	for (let i = 0; i < args.length; i++) {
		switch (args[i]) {
			case '--red': opts.redDifficulty = parseInt(args[++i], 10); break;
			case '--blue': opts.blueDifficulty = parseInt(args[++i], 10); break;
			case '--tc': opts.timeControl = args[++i]; break;
			case '--max-moves': opts.maxMoves = parseInt(args[++i], 10); break;
			case '--json': opts.json = true; break;
			case '--help':
				console.log('Usage: node scripts/simulate-match.mjs [--red N] [--blue N] [--tc TIME] [--max-moves N] [--json]');
				console.log('');
				console.log('Options:');
				console.log('  --red N        Red AI difficulty (1-5, default 5)');
				console.log('  --blue N       Blue AI difficulty (1-5, default 3)');
				console.log('  --tc TIME      Time control (default 7+5)');
				console.log('  --max-moves N  Max moves before declaring draw (default 200)');
				console.log('  --json         Output result as JSON');
				process.exit(0);
		}
	}
	return opts;
}

const NAMES = ['', 'Novice', 'Beginner', 'Intermediate', 'Advanced', 'Grandmaster'];

async function main() {
	const opts = parseArgs();
	if (!opts.json) {
		console.log(`=== Caro AI PvP - Simulate Match ===`);
		console.log(`Red:  L${opts.redDifficulty} (${NAMES[opts.redDifficulty]})`);
		console.log(`Blue: L${opts.blueDifficulty} (${NAMES[opts.blueDifficulty]})`);
		console.log(`TC: ${opts.timeControl}`);
		console.log();
	}

	// Create game with per-player difficulty
	const createResp = await fetch(`${API_BASE}/api/game/new`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			timeControl: opts.timeControl,
			gameMode: 'aivai',
			redDifficulty: opts.redDifficulty,
			blueDifficulty: opts.blueDifficulty
		})
	});
	if (!createResp.ok) throw new Error(`Create game failed: ${await createResp.text()}`);
	const { gameId } = await createResp.json();

	if (!opts.json) console.log(`Game: ${gameId}`);

	const startTime = Date.now();
	let moveCount = 0;
	let winner = null;
	let reason = '';

	while (moveCount < opts.maxMoves) {
		const moveResp = await fetch(`${API_BASE}/api/game/${gameId}/ai-move`, {
			method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}'
		});
		if (!moveResp.ok) {
			const errorText = await moveResp.text();
			console.error(`AI move failed at move ${moveCount + 1} (HTTP ${moveResp.status}): ${errorText}`);
			throw new Error(`AI move failed at move ${moveCount + 1} (HTTP ${moveResp.status}): ${errorText}`);
		}
		const data = await moveResp.json();
		moveCount++;

		if (!opts.json && moveCount % 10 === 0) {
			process.stdout.write(`Move ${moveCount}...\r`);
		}

		if (data.state.isGameOver) {
			winner = data.state.winner;
			reason = winner ? 'win' : 'draw';
			break;
		}
	}
	if (!winner && moveCount >= opts.maxMoves) reason = 'max-moves';
	const elapsed = ((Date.now() - startTime) / 1000).toFixed(1);

	if (opts.json) {
		console.log(JSON.stringify({
			gameId,
			redDifficulty: opts.redDifficulty,
			blueDifficulty: opts.blueDifficulty,
			timeControl: opts.timeControl,
			moves: moveCount,
			winner: winner || 'none',
			reason,
			elapsedSeconds: parseFloat(elapsed)
		}, null, 2));
	} else {
		console.log(`\nResult: ${winner ? winner.toUpperCase() + ' wins' : 'Draw'} (${reason})`);
		console.log(`Moves: ${moveCount} in ${elapsed}s`);
	}
}

main().catch(err => { console.error(`Fatal: ${err.message}`); process.exit(1); });
