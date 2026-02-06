import type { Cell, Player, GameState } from '$lib/types/game';
import { UCIEngine, movesToUCI, fromUCI, toUCI } from '$lib/uciEngine';

export interface MoveRecord {
	moveNumber: number;
	player: Player;
	x: number;
	y: number;
}

export class GameStore {
	board = $state<Cell[]>([]);
	currentPlayer = $state<Player>('red');
	moveNumber = $state(0);
	isGameOver = $state(false);
	winner = $state<Player | undefined>(undefined);
	moveHistory = $state<MoveRecord[]>([]);
	
	// UCI engine integration
	uciEngine: UCIEngine | null = null;
	uciConnected = $state(false);
	useUCI = $state(false);

	constructor() {
		this.reset();
	}

	reset() {
		this.board = Array.from({ length: 225 }, (_, i) => ({
			x: i % 15,
			y: Math.floor(i / 15),
			player: 'none' as Player
		}));
		this.currentPlayer = 'red';
		this.moveNumber = 0;
		this.isGameOver = false;
		this.winner = undefined;
		this.moveHistory = [];
	}

	makeMove(x: number, y: number): boolean {
		if (this.isGameOver) return false;

		const cell = this.board.find((c) => c.x === x && c.y === y);
		if (!cell || cell.player !== 'none') return false;

		// Record move to history before changing player
		this.moveHistory.push({
			moveNumber: this.moveNumber + 1,
			player: this.currentPlayer,
			x,
			y
		});

		cell.player = this.currentPlayer;
		this.moveNumber++;
		this.currentPlayer = this.currentPlayer === 'red' ? 'blue' : 'red';

		return true;
	}

	/**
	 * Connect to the UCI engine.
	 */
	async connectUCI(url = 'ws://localhost:5207/ws/uci'): Promise<boolean> {
		if (this.uciEngine) {
			this.uciEngine.disconnect();
		}

		this.uciEngine = new UCIEngine(url);
		
		try {
			this.uciConnected = await this.uciEngine.connect();
			if (this.uciConnected) {
				await this.uciEngine.initialize();
			}
			return this.uciConnected;
		} catch (error) {
			console.error('[GameStore] Failed to connect to UCI engine:', error);
			this.uciConnected = false;
			return false;
		}
	}

	/**
	 * Disconnect from the UCI engine.
	 */
	disconnectUCI() {
		if (this.uciEngine) {
			this.uciEngine.disconnect();
			this.uciEngine = null;
		}
		this.uciConnected = false;
	}

	/**
	 * Get AI move via UCI engine.
	 * Returns the move in {x, y} format.
	 */
	async getAIMoveUCI(): Promise<{ x: number; y: number } | null> {
		if (!this.uciEngine || !this.uciConnected) {
			console.warn('[GameStore] UCI engine not connected');
			return null;
		}

		try {
			// Convert move history to UCI format
			const uciMoves = movesToUCI(this.moveHistory);
			
			// Get best move from engine
			const uciMove = await this.uciEngine.getBestMoveAsync(uciMoves);
			
			// Convert back to coordinates
			const move = fromUCI(uciMove);
			
			// Validate move is within 15x15 board
			if (move.x >= 15 || move.y >= 15) {
				console.warn('[GameStore] UCI move outside 15x15 board:', uciMove);
				return null;
			}

			return move;
		} catch (error) {
			console.error('[GameStore] Failed to get AI move from UCI:', error);
			return null;
		}
	}

	/**
	 * Enable or disable UCI mode.
	 */
	setUseUCI(enabled: boolean) {
		this.useUCI = enabled;
		if (enabled && !this.uciConnected) {
			// Try to connect when enabling UCI
			this.connectUCI();
		}
	}
}
