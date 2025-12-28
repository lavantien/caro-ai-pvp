import type { Cell, Player, GameState } from '$lib/types/game';

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
}
