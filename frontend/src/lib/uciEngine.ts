/**
 * UCI Engine client for WebSocket communication with the backend.
 * Speaks plaintext UCI lines (the backend's uci package format):
 * commands like "position startpos moves hh", replies like "uciok",
 * "readyok", "bestmove hh". Double-letter notation: letter(y) + letter(x).
 */

import { UCIConfig } from '$lib/config';

// Valid coordinate letters span 'a' through the board width (a-p on 16
// columns); derived so a board-size change cannot desync the parser.
const firstCoordLetter = String.fromCharCode(UCIConfig.asciiLowerA);
const lastCoordLetter = String.fromCharCode(UCIConfig.asciiLowerA + UCIConfig.maxRow - 1);
const coordLetterPattern = new RegExp(`[${firstCoordLetter}-${lastCoordLetter}]`);

export interface UCIResponse {
	id?: string[];
	options?: string[];
	uciOk?: boolean;
	readyOk?: boolean;
	ok?: boolean;
	stopped?: boolean;
	bestMove?: string;
	info?: UCIInfo;
	error?: string;
}

export interface UCIInfo {
	depth: number;
	nodes: number;
	timeMs: number;
	score: number;
	pv: string[];
}

interface LineWaiter {
	match: (line: string) => boolean;
	resolve: (line: string) => void;
	reject: (error: Error) => void;
	timer: ReturnType<typeof setTimeout>;
}

export class UCIEngine {
	private ws: WebSocket | null = null;
	private url: string;
	private connected = false;
	private waiters: LineWaiter[] = [];

	constructor(url: string = UCIConfig.defaultWsUrl) {
		this.url = url;
	}

	/**
	 * Connect to the UCI engine via WebSocket.
	 */
	async connect(): Promise<boolean> {
		if (this.ws?.readyState === WebSocket.OPEN) {
			return true;
		}

		return new Promise((resolve, reject) => {
			try {
				this.ws = new WebSocket(this.url);

				this.ws.onopen = () => {
					this.connected = true;
					resolve(true);
				};

				this.ws.onmessage = (event) => {
					const data = typeof event.data === 'string' ? event.data : '';
					for (const line of data.split('\n')) {
						if (line.trim()) this.handleLine(line.trim());
					}
				};

				this.ws.onerror = () => {
					this.connected = false;
					reject(new Error('WebSocket connection failed'));
				};

				this.ws.onclose = () => {
					this.connected = false;
					this.failWaiters(new Error('UCI engine connection closed'));
				};
			} catch (error) {
				reject(error);
			}
		});
	}

	/**
	 * Disconnect from the UCI engine.
	 */
	disconnect() {
		if (this.ws) {
			this.ws.close();
			this.ws = null;
			this.connected = false;
		}
		this.failWaiters(new Error('UCI engine disconnected'));
	}

	/**
	 * Check if connected to the engine.
	 */
	isConnected(): boolean {
		return this.connected && this.ws?.readyState === WebSocket.OPEN;
	}

	/**
	 * Initialize the UCI protocol handshake.
	 */
	async initialize(): Promise<UCIResponse> {
		this.send('uci');
		await this.waitFor((l) => l === 'uciok', UCIConfig.searchTimeoutMs);
		return { uciOk: true };
	}

	/**
	 * Check if the engine is ready.
	 */
	async isReady(): Promise<boolean> {
		this.send('isready');
		await this.waitFor((l) => l === 'readyok', UCIConfig.searchTimeoutMs);
		return true;
	}

	/**
	 * Start a new game.
	 */
	async newGame(): Promise<UCIResponse> {
		this.send('ucinewgame');
		return { ok: true };
	}

	/**
	 * Set position and optionally apply moves.
	 */
	async setPosition(position: string = 'startpos', moves?: string[]): Promise<UCIResponse> {
		const parts = ['position', position];
		if (moves?.length) {
			parts.push('moves', ...moves);
		}
		this.send(parts.join(' '));
		return { ok: true };
	}

	/**
	 * Get the best move for the given move history in double-letter notation.
	 */
	async getBestMoveAsync(
		moves: string[] = [],
		whiteTime = UCIConfig.defaultTimeMs,
		blackTime = UCIConfig.defaultTimeMs,
		whiteIncrement = UCIConfig.defaultIncrementMs,
		blackIncrement = UCIConfig.defaultIncrementMs
	): Promise<string> {
		await this.setPosition('startpos', moves);
		this.send(
			`go wtime ${whiteTime} btime ${blackTime} winc ${whiteIncrement} binc ${blackIncrement}`
		);
		const line = await this.waitFor(
			(l) => l.startsWith('bestmove '),
			UCIConfig.searchTimeoutMs
		);
		return line.slice('bestmove '.length).trim();
	}

	/** Alias kept for callers using the older name. */
	getBestMove = this.getBestMoveAsync;

	/**
	 * Set an engine option.
	 */
	async setOption(name: string, value: string | number | boolean): Promise<UCIResponse> {
		this.send(`setoption name ${name} value ${String(value)}`);
		return { ok: true };
	}

	/**
	 * Stop the current search. The engine will still answer with bestmove.
	 */
	async stop(): Promise<UCIResponse> {
		this.send('stop');
		return { stopped: true };
	}

	private send(line: string) {
		if (!this.isConnected() || !this.ws) {
			throw new Error('Not connected to UCI engine');
		}
		this.ws.send(line);
	}

	private waitFor(match: (line: string) => boolean, timeoutMs: number): Promise<string> {
		return new Promise((resolve, reject) => {
			const waiter: LineWaiter = {
				match,
				resolve: (line) => {
					clearTimeout(waiter.timer);
					this.waiters = this.waiters.filter((w) => w !== waiter);
					resolve(line);
				},
				reject: (error) => {
					clearTimeout(waiter.timer);
					this.waiters = this.waiters.filter((w) => w !== waiter);
					reject(error);
				},
				timer: setTimeout(() => {
					waiter.reject(new Error('Search timeout'));
				}, timeoutMs)
			};
			this.waiters.push(waiter);
		});
	}

	private failWaiters(error: Error) {
		for (const w of this.waiters) {
			w.reject(error);
		}
		this.waiters = [];
	}

	private handleLine(line: string) {
		const waiter = this.waiters.find((w) => w.match(line));
		if (waiter) {
			waiter.resolve(line);
		}
	}
}

/**
 * Convert (x, y) coordinates to the engine's double-letter UCI notation:
 * first letter encodes the row (y), second the column (x). Matches the
 * backend's uci.MoveToString, e.g. (7, 7) becomes "hh".
 */
export function toUCI(x: number, y: number): string {
	const maxIndex = UCIConfig.maxRow - 1;
	if (x < 0 || x > maxIndex || y < 0 || y > maxIndex) {
		throw new Error(`Coordinates out of bounds: (${x}, ${y})`);
	}
	const rowLetter = String.fromCharCode(UCIConfig.asciiLowerA + y);
	const colLetter = String.fromCharCode(UCIConfig.asciiLowerA + x);
	return `${rowLetter}${colLetter}`;
}

/**
 * Convert double-letter UCI notation to (x, y) coordinates.
 * "hh" becomes (7, 7); first letter is the row, second the column.
 */
export function fromUCI(move: string): { x: number; y: number } {
	if (!move || move.length !== 2) {
		throw new Error(`Invalid move: ${move} (expected two letters, e.g. hh)`);
	}

	move = move.toLowerCase();
	if (!coordLetterPattern.test(move[0]) || !coordLetterPattern.test(move[1])) {
		throw new Error(
			`Invalid coordinates in move: ${move} (expected ${firstCoordLetter}-${lastCoordLetter} for both letters)`
		);
	}

	const y = move.charCodeAt(0) - UCIConfig.asciiLowerA;
	const x = move.charCodeAt(1) - UCIConfig.asciiLowerA;

	return { x, y };
}

/**
 * Get a list of UCI moves from the game state.
 */
export function movesToUCI(history: Array<{ x: number; y: number }>): string[] {
	return history.map((m) => toUCI(m.x, m.y));
}
