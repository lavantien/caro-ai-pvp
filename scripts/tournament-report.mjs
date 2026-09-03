/**
 * Console and markdown rendering for the round-robin runner. Pure summary
 * -> text transforms; the entry script owns file writes.
 */

import { DIFFICULTY_NAMES } from './lib.mjs';

const pct = (v) => (v === null || v === undefined) ? 'n/a' : `${(v * 100).toFixed(1)}%`;

/** End-of-run console ladder plus a one-liner per completed pairing. */
export function renderConsoleLadder(summary) {
	const lines = ['', '=== Ladder ==='];
	for (const row of summary.ladder.rows) {
		const wins = Object.entries(row.wins)
			.map(([lvl, n]) => `L${lvl}:${n}`)
			.join(' ');
		const rate = row.higherWinRateDecisive === null
			? 'n/a'
			: `${pct(row.higherWinRateDecisive)} higher (CI ${pct(row.higherWinRate95CI.low)}-${pct(row.higherWinRate95CI.high)})`;
		lines.push(`${row.label.padEnd(6)} games=${row.games} draws=${row.draws} wins=[${wins}] ${rate}`);
	}
	for (const s of summary.ladder.adjacent) {
		lines.push(`adjacent ${s.step}: higher level wins ${pct(s.higherWinRateDecisive)} of decisive games`);
	}
	lines.push(`Monotonicity: ${summary.ladder.monotonicVerdict}`);
	return lines.join('\n');
}

function sideRow(level, side) {
	return `| L${level} ${DIFFICULTY_NAMES[level]} | ${side.moveCount} | ${side.meanDepth} | ${side.medianDepth} | ${side.meanNodes} | ${side.meanNps} | ${pct(side.meanTtHit)} | ${side.thinkOverAlloc} | ${side.vcf.moves} | ${side.ponder.withPonder}/${side.ponder.hits} |`;
}

function pairingSection(p) {
	const out = ['', `## ${p.label} (${p.summary.games} game${p.summary.games === 1 ? '' : 's'})`, ''];
	out.push(`Wins ${Object.entries(p.summary.wins).map(([l, n]) => `L${l}=${n}`).join(', ')}` +
		`, draws=${p.summary.draws}, avg moves=${p.summary.avgMoves}, avg ${p.summary.avgSeconds}s`);
	out.push(`End reasons: ${JSON.stringify(p.summary.reasons)}`, '');
	out.push('| side | moves | mean d | median d | mean nodes | mean nps | tt hit | think/alloc | vcf moves | ponder w/h |');
	out.push('|---|---|---|---|---|---|---|---|---|---|');
	for (const level of [...new Set([p.red, p.blue])]) {
		out.push(sideRow(level, p.sides[level]));
	}
	out.push('', '| # | seed | red | blue | winner | reason | moves | seconds |');
	out.push('|---|---|---|---|---|---|---|---|');
	for (const g of p.games) {
		out.push(`| ${g.index} | ${g.seed} | L${g.redDiff} | L${g.blueDiff} | ${g.winner} | ${g.reason} | ${g.movesPlayed} | ${g.elapsedSeconds.toFixed(1)} |`);
	}
	return out;
}

/** Full report.md body: header, determinism statement, ladder, per-pairing detail, anomalies. */
export function renderReportMarkdown(summary) {
	const { run, config, totals, anomalies } = summary;
	const out = ['# Round-robin tournament report', ''];
	out.push(`status: ${summary.status}${summary.error ? ` (${summary.error})` : ''}`, '');
	out.push(`started ${run.startedAt}; generated ${new Date().toISOString()}`, '');

	out.push('## Run', '');
	out.push(`- git: ${run.git.commit ?? 'unknown'}${run.git.dirty ? ' (dirty working tree)' : ''}`);
	out.push(`- build: ${run.buildConfig} | node ${run.host.node} | ${run.host.cpus} CPUs | ${run.host.platform}`);
	out.push(`- argv: \`${run.argv.join(' ')}\``);
	out.push(`- games: ${totals.played}/${config.totalGames} played | ${totals.movesPlayed} moves | ${totals.elapsedSeconds.toFixed(0)}s elapsed`, '');

	out.push('## Determinism', '');
	out.push(`Openings are splitmix64-seeded from base seed ${config.baseSeed}: each game uses seed = base + global game index (${config.seedFirst}-${config.seedLast}). Pairing order is fixed, games run sequentially, colors swap every second game.`);
	out.push('');
	out.push('The `t=`, `nps=` statline columns and the achieved depth under a soft time budget are wall-clock evidence: they vary with machine load and are not part of the deterministic oracle. Multi-threaded levels (L3+) also vary in node counts run to run. Depth-capped levels (L1, L2) are deterministic.');
	out.push('');

	out.push('## Ladder', '');
	out.push('| pairing | games | draws | higher wins | decisive | higher win rate | 95% CI |');
	out.push('|---|---|---|---|---|---|---|');
	for (const row of summary.ladder.rows) {
		const higherWins = Object.entries(row.wins)
			.filter(([l]) => Number(l) === row.higher)
			.reduce((s, [, n]) => s + n, 0);
		out.push(`| ${row.label} | ${row.games} | ${row.draws} | L${row.higher}=${higherWins} | ${row.decisive} | ${pct(row.higherWinRateDecisive)} | ${pct(row.higherWinRate95CI?.low)}-${pct(row.higherWinRate95CI?.high)} |`);
	}
	out.push('', `Adjacent steps: ${summary.ladder.adjacent.map((s) => `${s.step} ${pct(s.higherWinRateDecisive)}`).join(', ') || 'none'}`);
	out.push('', `Verdict: ${summary.ladder.monotonicVerdict}`);

	for (const p of summary.pairings) {
		out.push(...pairingSection(p));
	}

	out.push('', '## Anomalies', '');
	out.push(`- timeout-fallback moves: ${anomalies.timeoutFallbackMoves}`);
	const fallbackByLevel = {};
	for (const p of summary.pairings) {
		for (const [level, side] of Object.entries(p.sides)) {
			const n = side.moveTypeCounts['timeout-fallback'] ?? 0;
			if (n > 0) fallbackByLevel[level] = (fallbackByLevel[level] ?? 0) + n;
		}
	}
	const fallbackEntries = Object.entries(fallbackByLevel).map(([l, n]) => `L${l}=${n}`).join(', ');
	if (fallbackEntries) out.push(`- timeout-fallback by level: ${fallbackEntries}`);
	out.push(`- games hitting the move cap: ${anomalies.maxMoveGames}`);
	out.push(`- errored games: ${anomalies.erroredGames}`);
	out.push('- L1vL1 and L5vL5 are calibration pairings: near-balanced results are expected there, decisive skew in cross pairings.');
	out.push('');
	out.push('Timeout-fallback moves, draws, and max-move games are recorded, not fatal; any other game failure aborts the run with a partial summary.');
	out.push('', '## Artifacts', '');
	out.push('- `run.log`: every banner and statline verbatim, plus the backend `move-statline` log lines.');
	out.push(`- \`summary.json\`: this report's source data (schema v${summary.schemaVersion}).`);
	out.push('- `matches.db`: sqlite archive of games and moves. Example query:');
	out.push('');
	out.push('```sql');
	out.push('SELECT difficulty, COUNT(*) vcf_moves, AVG(vcf_depth) avg_chain');
	out.push('FROM moves WHERE move_type = \'vcf\' GROUP BY difficulty ORDER BY difficulty;');
	out.push('```');

	return out.join('\n') + '\n';
}
