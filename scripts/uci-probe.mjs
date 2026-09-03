#!/usr/bin/env node

/**
 * UCI probe — deterministic engine probes over the /ws/uci WebSocket bridge.
 *
 * Modes:
 *   parity — fixed positions at fixed depths, single thread, fixed hash;
 *            outputs are recorded for cross-engine comparison.
 *   speed  — full depth-N search on the midgame position under a large
 *            fixed clock, reports nodes/nps.
 *
 * Usage:
 *   node scripts/uci-probe.mjs --mode parity
 *   node scripts/uci-probe.mjs --mode speed
 */

import { readFileSync } from 'node:fs';
import { ARTIFACTS, PROBE, WS_UCI_URL } from './lib.mjs';

function parseArgs(argv) {
	const args = { mode: 'parity', url: WS_UCI_URL };
	for (let i = 0; i < argv.length; i++) {
		if (argv[i] === '--mode' && argv[i + 1]) args.mode = argv[++i];
		else if (argv[i] === '--url' && argv[i + 1]) args.url = argv[++i];
	}
	return args;
}

class UciClient {
	constructor(url) {
		this.url = url;
		this.lines = [];
		this.waiters = [];
		this.socket = null;
	}

	connect(timeoutMs = PROBE.connectTimeoutMs) {
		return new Promise((resolvePromise, reject) => {
			const socket = new WebSocket(this.url);
			this.socket = socket;
			const timer = setTimeout(() => reject(new Error(`connect timeout: ${this.url}`)), timeoutMs);
			socket.onopen = () => { clearTimeout(timer); resolvePromise(); };
			socket.onerror = () => { clearTimeout(timer); reject(new Error(`connect failed: ${this.url}`)); };
			socket.onmessage = (event) => {
				for (const line of String(event.data).split('\n')) {
					const trimmed = line.trim();
					if (!trimmed) continue;
					const waiter = this.waiters.find((w) => w.predicate(trimmed));
					if (waiter) {
						this.waiters.splice(this.waiters.indexOf(waiter), 1);
						waiter.resolve(trimmed);
					} else {
						this.lines.push(trimmed);
					}
				}
			};
			socket.onclose = () => {
				for (const w of this.waiters.splice(0)) w.reject(new Error('socket closed'));
			};
		});
	}

	send(line) {
		this.socket.send(line);
	}

	/** Resolves with the first line matching predicate (consumed lines are dropped). */
	waitFor(predicate, timeoutMs = PROBE.waitLineTimeoutMs) {
		const existing = this.lines.findIndex(predicate);
		if (existing >= 0) return Promise.resolve(this.lines.splice(existing, 1)[0]);
		return new Promise((resolvePromise, reject) => {
			const timer = setTimeout(() => {
				const idx = this.waiters.indexOf(waiter);
				if (idx >= 0) this.waiters.splice(idx, 1);
				reject(new Error(`timeout waiting for line (have: ${this.lines.slice(-3).join(' | ')})`));
			}, timeoutMs);
			const waiter = {
				predicate,
				resolve: (line) => { clearTimeout(timer); resolvePromise(line); },
			};
			this.waiters.push(waiter);
		});
	}

	async command(line, expect, timeoutMs) {
		this.send(line);
		if (!expect) return;
		return this.waitFor(expect, timeoutMs);
	}

	close() {
		try { this.socket.close(); } catch { /* already closed */ }
	}
}

function parseInfo(line) {
	const info = {};
	const tokens = line.split(/\s+/);
	for (let i = 0; i + 1 < tokens.length; i++) {
		switch (tokens[i]) {
			case 'depth': info.depth = Number(tokens[i + 1]); break;
			case 'nodes': info.nodes = Number(tokens[i + 1]); break;
			case 'nps': info.nps = Number(tokens[i + 1]); break;
			case 'score': if (tokens[i + 1] === 'cp') info.scoreCp = Number(tokens[i + 2]); break;
			case 'tt-hitrate': info.ttHitRate = Number(tokens[i + 1]); break;
			case 'threads': info.threads = Number(tokens[i + 1]); break;
		}
	}
	return info;
}

async function handshake(client) {
	await client.command('uci', (l) => l === 'uciok');
	await client.command(`setoption name Threads value ${PROBE.threads}`);
	await client.command(`setoption name Hash value ${PROBE.hashMB}`);
	await client.command(`setoption name Skill Level value ${PROBE.skill}`);
	await client.command('isready', (l) => l === 'readyok');
}

async function probePosition(client, position, depth, timeoutMs) {
	await client.command('ucinewgame', null);
	await client.command(`position startpos moves ${position.moves.join(' ')}`, null);
	// A bare "go depth N" leaves the clock at 0 and the search aborts
	// instantly, so give the side to move a large fixed clock: depth is then
	// the only binding limit and the search is deterministic.
	const side = position.moves.length % 2 === 0 ? 'w' : 'b';
	await client.command(`go depth ${depth} ${side}time ${PROBE.clockMs}`, null, timeoutMs);
	const infoLine = await client.waitFor((l) => l.startsWith('info '), timeoutMs);
	const bestMove = await client.waitFor((l) => l.startsWith('bestmove '), timeoutMs);
	return { ...parseInfo(infoLine), bestmove: bestMove.split(' ')[1] };
}

async function runParity(client, positions) {
	console.log(`# parity: single thread, hash ${PROBE.hashMB}, skill ${PROBE.skill}, fixed depth`);
	console.log('# position\tdepth\tbestmove\tscoreCp\tnodes\tnps\tttHitRate');
	for (const position of positions) {
		for (const depth of position.depths) {
			const result = await probePosition(client, position, depth, PROBE.parityTimeoutMs);
			console.log(
				`${position.name}\t${depth}\t${result.bestmove}\t${result.scoreCp}\t` +
				`${result.nodes}\t${result.nps}\t${result.ttHitRate}`
			);
		}
	}
}

async function runSpeed(client, positions) {
	const position = positions.find((p) => p.speed) ?? positions[1] ?? positions[0];
	console.log(`# speed: single thread, full depth-${PROBE.speedDepth} search, nodes/nps from info line`);
	console.log('# position\trun\tnodes\tnps\tdepth');
	const side = position.moves.length % 2 === 0 ? 'w' : 'b';
	for (let run = 1; run <= PROBE.speedRuns; run++) {
		await client.command('ucinewgame', null);
		await client.command(`position startpos moves ${position.moves.join(' ')}`, null);
		await client.command(`go depth ${PROBE.speedDepth} ${side}time ${PROBE.clockMs}`, null, PROBE.speedTimeoutMs);
		const infoLine = await client.waitFor((l) => l.startsWith('info '), PROBE.speedTimeoutMs);
		await client.waitFor((l) => l.startsWith('bestmove '), PROBE.speedTimeoutMs);
		const result = parseInfo(infoLine);
		console.log(`${position.name}\t${run}\t${result.nodes}\t${result.nps}\t${result.depth}`);
	}
}

async function main() {
	const args = parseArgs(process.argv.slice(2));
	const { positions } = JSON.parse(readFileSync(ARTIFACTS.positions, 'utf8'));
	const client = new UciClient(args.url);
	await client.connect();
	try {
		await handshake(client);
		if (args.mode === 'speed') await runSpeed(client, positions);
		else await runParity(client, positions);
	} finally {
		client.close();
	}
}

main().catch((err) => {
	console.error(`Fatal: ${err.message}`);
	process.exit(1);
});
