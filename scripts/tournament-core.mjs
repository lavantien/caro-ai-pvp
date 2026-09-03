/**
 * Pure game-play and aggregation logic for the round-robin runner
 * (scripts/run-round-robin.mjs). No process or stream management here:
 * the entry script owns the backend lifecycle and the artifact files.
 *
 * Aggregation reads only the structured engineStats payload, never the
 * statline text; the statline is echoed verbatim into run.log as evidence.
 */

import { renameSync, writeFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import os from 'node:os';
import {
	API_BASE_URL,
	ENDPOINTS,
	GAME_MODE_AIVAI,
	ROUND_ROBIN_PAIRINGS,
	ROOT,
	postJson
} from './lib.mjs';

// --- CLI parsing helpers ---

/** Validates "LvL" tokens, returns the selected pairings in canonical order. */
export function parsePairingFilter(spec) {
	if (!spec) return ROUND_ROBIN_PAIRINGS.map((p) => [...p]);
	const wanted = new Set();
	for (const raw of spec.split(',')) {
		const token = raw.trim();
		const m = /^([1-5])v([1-5])$/.exec(token);
		if (!m) throw new Error(`Invalid pairing "${token}" (expected LvL like 1v5)`);
		const key = `${m[1]}v${m[2]}`;
		if (!ROUND_ROBIN_PAIRINGS.some(([a, b]) => `${a}v${b}` === key)) {
			throw new Error(`Pairing ${key} is not in the canonical round-robin list`);
		}
		wanted.add(key);
	}
	return ROUND_ROBIN_PAIRINGS.filter(([a, b]) => wanted.has(`${a}v${b}`)).map((p) => [...p]);
}

// --- Game play ---

function toMoveRecord(last) {
	return {
		moveNumber: last.moveNumber,
		player: last.player,
		statline: last.statline,
		thinkTimeMs: last.thinkTimeMs,
		ponderHit: last.ponderHit ?? null,
		engineStats: { ...(last.engineStats ?? {}) }
	};
}

async function getState(gameId) {
	const resp = await fetch(`${API_BASE_URL}${ENDPOINTS.get(gameId)}`);
	if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${(await resp.text()).slice(0, 300)}`);
	return (await resp.json()).state;
}

/**
 * Plays one seeded aivai game to completion.
 *
 * Timeout adjudication surfaces as HTTP 400 on the next touch (the session
 * only flags players on access), so a failed ai-move is retried against a
 * recovery GET: if the authoritative state says the game is over the result
 * is recorded; otherwise the error propagates (fail fast).
 */
export async function playGame({ redDiff, blueDiff, tc, maxMoves, seed, onMove }) {
	const { gameId } = await postJson(`${API_BASE_URL}${ENDPOINTS.newGame}`, {
		timeControl: tc,
		gameMode: GAME_MODE_AIVAI,
		redDifficulty: redDiff,
		blueDifficulty: blueDiff,
		randomOpening: true,
		seed
	});

	const startedAt = Date.now();
	const moves = [];
	let winner = null;
	let reason = '';

	try {
		while (moves.length < maxMoves) {
			let data;
			try {
				data = await postJson(`${API_BASE_URL}${ENDPOINTS.aiMove(gameId)}`);
			} catch (err) {
				const state = await getState(gameId);
				if (!state.isGameOver) throw err;
				winner = state.winner || 'none';
				reason = state.endReason || 'win';
				break;
			}
			if (data.lastMove?.statline) {
				onMove?.(data.lastMove);
				moves.push(toMoveRecord(data.lastMove));
			}
			if (data.state.isGameOver) {
				winner = data.state.winner || 'none';
				reason = data.state.endReason || (winner === 'none' ? 'draw' : 'win');
				break;
			}
		}
	} finally {
		// Completes the matches.db row and frees engine memory regardless of outcome.
		await fetch(`${API_BASE_URL}${ENDPOINTS.delete(gameId)}`, { method: 'DELETE' }).catch(() => {});
	}

	if (winner === null) {
		winner = 'none';
		reason = 'max-moves';
	}

	return {
		gameId,
		moves,
		winner,
		reason,
		movesPlayed: moves.length,
		elapsedSeconds: (Date.now() - startedAt) / 1000,
		timeoutFallbackMoves: moves.filter((m) => m.engineStats.moveType === 'timeout-fallback').length
	};
}

// --- Statistics ---

export function mean(xs) {
	return xs.length ? xs.reduce((s, v) => s + v, 0) / xs.length : 0;
}

export function median(xs) {
	if (!xs.length) return 0;
	const s = [...xs].sort((a, b) => a - b);
	const mid = s.length >> 1;
	return s.length % 2 ? s[mid] : (s[mid - 1] + s[mid]) / 2;
}

/** 95% Wilson score interval for a proportion. */
export function wilsonInterval(wins, n) {
	if (n === 0) return { low: 0, high: 1 };
	const z = 1.959963984540054;
	const p = wins / n;
	const denom = 1 + (z * z) / n;
	const center = (p + (z * z) / (2 * n)) / denom;
	const half = (z * Math.sqrt((p * (1 - p)) / n + (z * z) / (4 * n * n))) / denom;
	return { low: Math.max(0, center - half), high: Math.min(1, center + half) };
}

const round = (v, digits = 2) => (Number.isFinite(v) ? Math.round(v * 10 ** digits) / 10 ** digits : 0);
const nums = (xs) => xs.filter((v) => typeof v === 'number' && Number.isFinite(v));

/**
 * Aggregates all moves played by one difficulty level within a pairing.
 * Depth/nodes/nps/tt rows exclude VCF solver moves (d=0 n=0 by contract),
 * which are counted separately so they do not skew the search means.
 */
export function aggregateSide(moves) {
	const searched = moves.filter((m) => m.engineStats.moveType !== 'vcf');
	const pick = (f) => nums(searched.map((m) => f(m.engineStats)));
	const moveTypeCounts = {};
	for (const m of moves) {
		const t = m.engineStats.moveType || 'exact';
		moveTypeCounts[t] = (moveTypeCounts[t] || 0) + 1;
	}
	const vcfMoves = moves.filter((m) => m.engineStats.moveType === 'vcf');
	const pondered = moves.filter((m) => m.ponderHit !== null);
	const ratios = nums(searched.map((m) =>
		m.engineStats.allocatedTimeMs > 0 ? m.thinkTimeMs / m.engineStats.allocatedTimeMs : NaN));

	return {
		moveCount: moves.length,
		meanDepth: round(mean(pick((e) => e.depth))),
		medianDepth: round(median(pick((e) => e.depth))),
		meanNodes: round(mean(pick((e) => e.nodes))),
		meanNps: round(mean(pick((e) => e.nps))),
		meanTtHit: round(mean(pick((e) => e.ttHitRate)), 4),
		meanThinkMs: round(mean(nums(searched.map((m) => m.thinkTimeMs)))),
		meanAllocMs: round(mean(pick((e) => e.allocatedTimeMs))),
		thinkOverAlloc: round(mean(ratios), 4),
		moveTypeCounts,
		vcf: {
			moves: vcfMoves.length,
			meanChainDepth: round(mean(nums(vcfMoves.map((m) => m.engineStats.vcfDepth)))),
			meanNodes: round(mean(nums(vcfMoves.map((m) => m.engineStats.vcfNodes))))
		},
		ponder: {
			withPonder: pondered.length,
			hits: pondered.filter((m) => m.ponderHit === true).length,
			meanDepth: round(mean(nums(pondered.map((m) => m.engineStats.ponderDepth)))),
			meanNodes: round(mean(nums(pondered.map((m) => m.engineStats.ponderNodes))))
		}
	};
}

/** Wins by difficulty level, end-reason histogram, and the decisive-game rate. */
export function aggregatePairing(games) {
	const wins = {};
	const reasons = {};
	let draws = 0;
	for (const g of games) {
		if (g.winner === 'none') {
			draws++;
		} else {
			const level = g.winner === 'red' ? g.redDiff : g.blueDiff;
			wins[level] = (wins[level] || 0) + 1;
		}
		reasons[g.reason] = (reasons[g.reason] || 0) + 1;
	}
	const decisive = games.length - draws;
	const redWins = games.filter((g) => g.winner === 'red').length;
	const ci = wilsonInterval(redWins, decisive);
	return {
		games: games.length,
		wins,
		draws,
		reasons,
		avgMoves: round(mean(games.map((g) => g.movesPlayed))),
		avgSeconds: round(mean(games.map((g) => g.elapsedSeconds)), 1),
		redWinRateDecisive: decisive > 0 ? round(redWins / decisive, 4) : null,
		redWinRate95CI: decisive > 0 ? { low: round(ci.low, 4), high: round(ci.high, 4) } : null
	};
}

/**
 * Ladder rows plus adjacent-level steps (1v2, 2v3, 3v4, 4v5). Verdict is
 * monotonic when every measured step favors the higher level on decisive
 * games; a 1v1 / 5v5 calibration pairing contributes no step.
 */
export function buildLadder(pairings) {
	const rows = pairings.map((p) => {
		const { red: a, blue: b } = p;
		const decisive = p.summary.games - p.summary.draws;
		const higher = Math.max(a, b);
		const higherWins = a === b ? p.summary.wins[a] ?? 0 : p.summary.wins[higher] ?? 0;
		const ci = wilsonInterval(higherWins, decisive);
		return {
			label: `L${a}vL${b}`,
			red: a,
			blue: b,
			games: p.summary.games,
			wins: p.summary.wins,
			draws: p.summary.draws,
			decisive,
			higher,
			higherWinRateDecisive: decisive > 0 ? round(higherWins / decisive, 4) : null,
			higherWinRate95CI: decisive > 0 ? { low: round(ci.low, 4), high: round(ci.high, 4) } : null
		};
	});

	const adjacent = [];
	for (const lo of [1, 2, 3, 4]) {
		const row = rows.find((r) => Math.min(r.red, r.blue) === lo && Math.max(r.red, r.blue) === lo + 1);
		if (row && row.higherWinRateDecisive !== null) {
			adjacent.push({ step: `L${lo}vL${lo + 1}`, higherWinRateDecisive: row.higherWinRateDecisive });
		}
	}
	const verdict = adjacent.length < 3
		? 'inconclusive (fewer than 3 adjacent steps measured)'
		: adjacent.every((s) => s.higherWinRateDecisive > 0.5)
			? 'monotonic (higher level favored on every adjacent step)'
			: 'non-monotonic (some adjacent step favors the lower level)';
	return { rows, adjacent, monotonicVerdict: verdict };
}

// --- Artifacts ---

export function writeAtomic(path, content) {
	const tmp = `${path}.tmp`;
	writeFileSync(tmp, content);
	renameSync(tmp, path);
}

export function writeJsonAtomic(path, value) {
	writeAtomic(path, JSON.stringify(value, null, 2));
}

/** Git state, host, and argv provenance for the run header. */
export function captureRunHeader({ buildConfig, argv }) {
	const git = { commit: null, dirty: null };
	try {
		git.commit = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: ROOT, encoding: 'utf8' }).stdout?.trim() ?? null;
		git.dirty = (spawnSync('git', ['status', '--porcelain'], { cwd: ROOT, encoding: 'utf8' }).stdout ?? '').trim().length > 0;
	} catch {
		// git unavailable; provenance stays null
	}
	return {
		git,
		host: { node: process.version, cpus: os.cpus().length, platform: process.platform },
		buildConfig,
		argv,
		startedAt: new Date().toISOString(),
		determinism: {
			openings: 'splitmix64 seeded, per-game seed = base + global game index',
			order: 'fixed pairing order, sequential games, color swap every second game',
			evidence: 't/nps columns and soft-budget depth are wall-clock evidence, not oracle'
		}
	};
}
