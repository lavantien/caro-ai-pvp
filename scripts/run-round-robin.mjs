#!/usr/bin/env node

/**
 * Round-robin AI benchmark: builds/starts the backend (Release by default)
 * and plays every difficulty pairing in a fixed order, 1v1 first (fail fast)
 * and 5v5 last (calibration). Writes per-run evidence into
 * docs/artifacts/tournaments/<label>/ (run.log, summary.json, report.md,
 * matches.db).
 *
 * Usage: node scripts/run-round-robin.mjs [--games-per-pairing N]
 *        [--tc TIME] [--seed N] [--label NAME] [--pairings 1v1,3v4]
 *        [--build Release|Debug] [--max-moves N] [--json]
 *
 * Examples:
 *   node scripts/run-round-robin.mjs
 *   node scripts/run-round-robin.mjs --pairings 1v1 --games-per-pairing 2 --tc 1+0 --label smoke-l1v1
 */

import { createWriteStream, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import {
	BOARD,
	DIFFICULTY_NAMES,
	GAMES_PER_PAIRING_DEFAULT,
	ROUND_ROBIN_BUILD_CONFIG,
	ROUND_ROBIN_TIME_CONTROL,
	TOURNAMENT_BASE_SEED,
	createProcessManager,
	startBackend,
	teeConsole,
	timeControl,
	tournamentDir
} from './lib.mjs';
import {
	aggregatePairing,
	aggregateSide,
	buildLadder,
	captureRunHeader,
	parsePairingFilter,
	playGame,
	writeAtomic,
	writeJsonAtomic
} from './tournament-core.mjs';
import { renderConsoleLadder, renderReportMarkdown } from './tournament-report.mjs';

const mgr = createProcessManager();

function parseArgs() {
	const args = process.argv.slice(2);
	const opts = {
		gamesPerPairing: GAMES_PER_PAIRING_DEFAULT,
		timeControl: ROUND_ROBIN_TIME_CONTROL,
		seed: TOURNAMENT_BASE_SEED,
		label: null,
		pairings: null,
		build: ROUND_ROBIN_BUILD_CONFIG,
		maxMoves: BOARD.totalCells,
		json: false
	};
	for (let i = 0; i < args.length; i++) {
		switch (args[i]) {
			case '--games-per-pairing': opts.gamesPerPairing = parseInt(args[++i], 10); break;
			case '--tc': opts.timeControl = args[++i]; break;
			case '--seed': opts.seed = parseInt(args[++i], 10); break;
			case '--label': opts.label = args[++i]; break;
			case '--pairings': opts.pairings = args[++i]; break;
			case '--build': {
				const v = args[++i];
				opts.build = v ? v[0].toUpperCase() + v.slice(1).toLowerCase() : '';
				break;
			}
			case '--max-moves': opts.maxMoves = parseInt(args[++i], 10); break;
			case '--json': opts.json = true; break;
			case '--help':
				console.log('Usage: node scripts/run-round-robin.mjs [options]');
				console.log('');
				console.log('Options:');
				console.log('  --games-per-pairing N  Games per pairing (default 20)');
				console.log('  --tc TIME             Time control (default 3+2)');
				console.log('  --seed N              Base seed; game seeds are base + global index (default 20260821)');
				console.log('  --label NAME          Run directory name under docs/artifacts/tournaments/');
				console.log('  --pairings LIST       Comma-separated LvL filter in canonical order (default all 12)');
				console.log('  --build CONFIG        Release or Debug (default Release)');
				console.log('  --max-moves N         Runner move cap safety net (default 256 = board cells)');
				console.log('  --json                Print summary JSON at the end');
				process.exit(0);
		}
	}
	if (!Number.isInteger(opts.gamesPerPairing) || opts.gamesPerPairing < 1) throw new Error('--games-per-pairing must be a positive integer');
	if (!Number.isInteger(opts.maxMoves) || opts.maxMoves < 1) throw new Error('--max-moves must be a positive integer');
	if (!Number.isInteger(opts.seed) || opts.seed < 0) throw new Error('--seed must be a non-negative integer');
	if (!['Release', 'Debug'].includes(opts.build)) throw new Error('--build must be Release or Debug');
	return opts;
}

function defaultLabel(seed) {
	const ts = new Date().toISOString().slice(0, 16); // "yyyy-mm-ddThh:mm"
	return `${ts.slice(0, 10).replace(/-/g, '')}-${ts.slice(11).replace(':', '')}-seed${seed}`;
}

async function main() {
	const opts = parseArgs();
	const label = opts.label ?? defaultLabel(opts.seed);
	const runDir = tournamentDir(label);
	mkdirSync(runDir, { recursive: true });

	// Tee everything (banners, statlines, backend log lines) after arg
	// parsing so --help never creates artifacts.
	const logStream = createWriteStream(join(runDir, 'run.log'), { flags: 'w' });
	teeConsole(logStream);
	const summaryPath = join(runDir, 'summary.json');
	const reportPath = join(runDir, 'report.md');

	const pairings = parsePairingFilter(opts.pairings);
	timeControl(opts.timeControl);
	const totalGames = pairings.length * opts.gamesPerPairing;

	const summary = {
		schemaVersion: 1,
		status: 'running',
		run: captureRunHeader({ buildConfig: opts.build, argv: process.argv.slice(2) }),
		config: {
			label,
			gamesPerPairing: opts.gamesPerPairing,
			timeControl: opts.timeControl,
			baseSeed: opts.seed,
			maxMoves: opts.maxMoves,
			build: opts.build,
			pairings,
			totalGames,
			seedFirst: opts.seed + 1,
			seedLast: opts.seed + totalGames
		},
		pairings: [],
		ladder: { rows: [], adjacent: [], monotonicVerdict: 'inconclusive (no pairings completed)' },
		totals: { planned: totalGames, played: 0, movesPlayed: 0, elapsedSeconds: 0 },
		anomalies: { timeoutFallbackMoves: 0, maxMoveGames: 0, erroredGames: 0 },
		error: null
	};

	const persist = () => {
		writeJsonAtomic(summaryPath, summary);
		writeAtomic(reportPath, renderReportMarkdown(summary));
	};

	console.log('=== Caro AI PvP - round-robin benchmark ===');
	console.log(`Run dir: ${runDir}`);
	console.log(`Pairings: ${pairings.map(([a, b]) => `L${a}vL${b}`).join(', ')}`);
	console.log(`Games per pairing: ${opts.gamesPerPairing} | TC: ${opts.timeControl} | Build: ${opts.build} | Base seed: ${opts.seed}`);
	console.log(`Total games: ${totalGames}`);
	console.log('');

	await startBackend(mgr, {
		buildConfig: opts.build,
		// Absolute on purpose: the backend daemon's cwd is backend/.
		env: { MATCH_DB_PATH: join(runDir, 'matches.db') }
	});

	persist();

	const runStart = Date.now();
	let g = 0;
	let aborted = null;

	gameLoop:
	for (const [a, b] of pairings) {
		console.log(`--- Pairing L${a}vL${b} (${DIFFICULTY_NAMES[a]} vs ${DIFFICULTY_NAMES[b]}) ---`);
		const pairingGames = [];
		const movesByLevel = { [a]: [], [b]: [] };

		for (let i = 1; i <= opts.gamesPerPairing; i++) {
			g++;
			const swap = i % 2 === 0;
			const redDiff = swap ? b : a;
			const blueDiff = swap ? a : b;
			const seed = opts.seed + g;
			console.log(`Game ${g}/${totalGames}: L${a}vL${b} #${i} red=L${redDiff} blue=L${blueDiff}${swap ? ' (swapped)' : ''} seed=${seed}`);

			let result;
			try {
				result = await playGame({
					redDiff,
					blueDiff,
					tc: opts.timeControl,
					maxMoves: opts.maxMoves,
					seed,
					onMove: (last) => console.log(last.statline)
				});
			} catch (err) {
				aborted = { game: g, pairing: `L${a}vL${b}`, message: err.message };
				summary.anomalies.erroredGames++;
				console.error(`  -> FATAL (aborting run): ${err.message}`);
				break gameLoop;
			}

			const levelWinner = result.winner === 'red' ? redDiff : result.winner === 'blue' ? blueDiff : null;
			pairingGames.push({
				index: g,
				seed,
				redDiff,
				blueDiff,
				swap,
				winner: result.winner,
				levelWinner,
				reason: result.reason,
				movesPlayed: result.movesPlayed,
				elapsedSeconds: Math.round(result.elapsedSeconds * 10) / 10,
				timeoutFallbackMoves: result.timeoutFallbackMoves,
				gameId: result.gameId
			});
			for (const m of result.moves) {
				movesByLevel[m.player === 'red' ? redDiff : blueDiff].push(m);
			}

			summary.totals.played++;
			summary.totals.movesPlayed += result.movesPlayed;
			summary.anomalies.timeoutFallbackMoves += result.timeoutFallbackMoves;
			if (result.reason === 'max-moves') summary.anomalies.maxMoveGames++;
			console.log(`  -> ${result.winner}${levelWinner !== null ? ` (L${levelWinner})` : ''} by ${result.reason} | ${result.movesPlayed} moves | ${result.elapsedSeconds.toFixed(1)}s`);
		}

		// Same-level pairings collapse to one side (both colors are that level
		// already, collected under the single surviving key).
		const sides = a === b
			? { [a]: aggregateSide(movesByLevel[a]) }
			: { [a]: aggregateSide(movesByLevel[a]), [b]: aggregateSide(movesByLevel[b]) };
		summary.pairings.push({
			red: a,
			blue: b,
			label: `L${a}vL${b}`,
			games: pairingGames,
			summary: aggregatePairing(pairingGames),
			sides
		});
		summary.ladder = buildLadder(summary.pairings);
		summary.totals.elapsedSeconds = (Date.now() - runStart) / 1000;
		persist();
		const done = summary.pairings.at(-1);
		console.log(`  Pairing done: wins=${JSON.stringify(done.summary.wins)} draws=${done.summary.draws} avg moves=${done.summary.avgMoves}`);
	}

	summary.totals.elapsedSeconds = (Date.now() - runStart) / 1000;
	if (aborted) {
		summary.status = 'aborted';
		summary.error = `game ${aborted.game} (${aborted.pairing}): ${aborted.message}`;
	} else {
		summary.status = 'completed';
	}
	persist();

	if (opts.json) {
		console.log(JSON.stringify(summary, null, 2));
	} else {
		console.log(renderConsoleLadder(summary));
	}
	console.log(`Artifacts: ${runDir}`);

	logStream.end();
	mgr.cleanup();
	process.exit(aborted ? 1 : 0);
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	mgr.cleanup();
	process.exit(1);
});
