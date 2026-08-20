/**
 * UCI Engine client for WebSocket communication with the backend.
 * Implements the Universal Chess Interface protocol for Caro.
 */

import { UCIConfig } from '$lib/config/uciConfig';

export interface UCICommand {
	command: string;
	position?: string;
	moves?: string[];
	whiteTime?: number;
	blackTime?: number;
	whiteIncrement?: number;
	blackIncrement?: number;
	moveTime?: number;
	depth?: number;
	nodes?: number;
	infinite?: boolean;
	name?: string;
	value?: string;
}

export interface UCIResponse {
	id?: string[];
	options?: string[];
	uciOk?: boolean;
	readyOk?: boolean;
	ok?: boolean;
	searching?: boolean;
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

export interface UCIMoveResult {
	bestMove: string;
	info?: UCIInfo;
}

/**
 * UCI Engine class for WebSocket-based UCI communication.
 */
export class UCIEngine {
	private ws: WebSocket | null = null;
	private url: string;
	private connected = $state(false);
	private pendingCommands = new Map<string, {
		resolve: (value: UCIResponse) => void;
		reject: (error: Error) => void;
	}>();
	private commandId = 0;
	private bestMoveCallback: ((move: string) => void) | null = null;

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
					console.log('[UCI] Connected to engine');
					this.connected = true;
					resolve(true);
				};

				this.ws.onmessage = (event) => {
					this.handleMessage(event.data);
				};

				this.ws.onerror = (error) => {
					console.error('[UCI] WebSocket error:', error);
					this.connected = false;
					reject(new Error('WebSocket connection failed'));
				};

				this.ws.onclose = () => {
					console.log('[UCI] Connection closed');
					this.connected = false;
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
	}

	/**
	 * Check if connected to the engine.
	 */
	isConnected(): boolean {
		return this.connected && this.ws?.readyState === WebSocket.OPEN;
	}

	/**
	 * Initialize UCI protocol.
	 */
	async initialize(): Promise<UCIResponse> {
		const response = await this.sendCommand({ command: 'uci' });
		if (response.uciOk) {
			console.log('[UCI] Engine initialized');
		}
		return response;
	}

	/**
	 * Check if engine is ready.
	 */
	async isReady(): Promise<boolean> {
		const response = await this.sendCommand({ command: 'isready' });
		return response.readyOk ?? false;
	}

	/**
	 * Start a new game.
	 */
	async newGame(): Promise<UCIResponse> {
		const response = await this.sendCommand({ command: 'ucinewgame' });
		return response;
	}

	/**
	 * Set position and optionally apply moves.
	 */
	async setPosition(position: string = 'startpos', moves?: string[]): Promise<UCIResponse> {
		return this.sendCommand({ command: 'position', position, moves });
	}

	/**
	 * Get the best move for the current position.
	 * Returns the move in UCI notation (e.g., "j10").
	 */
	async getBestMove(
		moves: string[] = [],
		whiteTime = UCIConfig.defaultTimeMs,
		blackTime = UCIConfig.defaultTimeMs,
		whiteIncrement = UCIConfig.defaultIncrementMs,
		blackIncrement = UCIConfig.defaultIncrementMs
	): Promise<string> {
		// Set position first
		await this.setPosition('startpos', moves);

		// Start search
		const response = await this.sendCommand({
			command: 'go',
			whiteTime,
			blackTime,
			whiteIncrement,
			blackIncrement
		});

		if (response.bestMove) {
			return response.bestMove;
		}

		throw new Error('No best move returned');
	}

	/**
	 * Get the best move as a promise that resolves when the search completes.
	 * This is useful for async/await patterns.
	 */
	async getBestMoveAsync(
		moves: string[] = [],
		whiteTime = UCIConfig.defaultTimeMs,
		blackTime = UCIConfig.defaultTimeMs,
		whiteIncrement = UCIConfig.defaultIncrementMs,
		blackIncrement = UCIConfig.defaultIncrementMs
	): Promise<string> {
		// Set position first
		await this.setPosition('startpos', moves);

		// Register a callback for the best move
		return new Promise((resolve, reject) => {
			this.bestMoveCallback = resolve;

			// Set up a timeout
			const timeout = setTimeout(() => {
				this.bestMoveCallback = null;
				reject(new Error('Search timeout'));
			}, UCIConfig.searchTimeoutMs);

			// Start search
			this.sendCommand({
				command: 'go',
				whiteTime,
				blackTime,
				whiteIncrement,
				blackIncrement
			}).then(() => {
				clearTimeout(timeout);
			}).catch((error) => {
				clearTimeout(timeout);
				this.bestMoveCallback = null;
				reject(error);
			});
		});
	}

	/**
	 * Set an engine option.
	 */
	async setOption(name: string, value: string | number | boolean): Promise<UCIResponse> {
		return this.sendCommand({
			command: 'setoption',
			name,
			value: String(value)
		});
	}


	/**
	 * Stop the current search.
	 */
	async stop(): Promise<UCIResponse> {
		return this.sendCommand({ command: 'stop' });
	}

	/**
	 * Send a command and wait for response.
	 */
	private async sendCommand(command: UCICommand): Promise<UCIResponse> {
		if (!this.isConnected()) {
			throw new Error('Not connected to UCI engine');
		}

		const id = `cmd_${++this.commandId}`;

		return new Promise((resolve, reject) => {
			this.pendingCommands.set(id, { resolve, reject });

			try {
				this.ws!.send(JSON.stringify(command));
			} catch (error) {
				this.pendingCommands.delete(id);
				reject(error);
			}
		});
	}

	/**
	 * Handle incoming WebSocket message.
	 */
	private handleMessage(data: string) {
		try {
			const response: UCIResponse = JSON.parse(data);

			// Check if this is a best move notification
			if (response.bestMove && this.bestMoveCallback) {
				const callback = this.bestMoveCallback;
				this.bestMoveCallback = null;
				callback(response.bestMove);
			}

			// For commands that expect a response, resolve with the first response
			// This is a simple implementation - in production, you might want more sophisticated matching
			if (this.pendingCommands.size > 0) {
				const firstEntry = this.pendingCommands.entries().next().value;
				if (firstEntry) {
					const [id, { resolve }] = firstEntry;
					this.pendingCommands.delete(id);
					resolve(response);
				}
			}
		} catch (error) {
			console.error('[UCI] Error parsing message:', error);
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
	if (!/[a-p]/.test(move[0]) || !/[a-p]/.test(move[1])) {
		throw new Error(`Invalid coordinates in move: ${move} (expected a-p for both letters)`);
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
