export type Player = 'none' | 'red' | 'blue';

export interface Cell {
	x: number;
	y: number;
	player: Player;
}

export interface GameState {
	board: Cell[];
	currentPlayer: Player;
	moveNumber: number;
	isGameOver: boolean;
	redTimeRemaining: number;
	blueTimeRemaining: number;
	winner?: Player;
}

export interface MoveRequest {
	x: number;
	y: number;
}

export interface GameCreatedResponse {
	gameId: string;
	state: GameState;
}
