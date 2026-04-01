export type Player = 'none' | 'red' | 'blue';

export function switchPlayer(player: Player): Player {
	return player === 'red' ? 'blue' : 'red';
}

export type TimeControl = '1+0' | '3+2' | '7+5' | '15+10';
export type GameMode = 'pvp' | 'pvai' | 'aivai';

export interface Cell {
	x: number;
	y: number;
	player: Player;
}

export interface GameCreateRequest {
	timeControl?: TimeControl;
	gameMode?: GameMode;
}

export interface GameState {
	board: Cell[];
	currentPlayer: Player;
	moveNumber: number;
	isGameOver: boolean;
	redTimeRemaining: number;
	blueTimeRemaining: number;
	winner?: Player;
	timeControl?: string;
	initialTime?: number;
	increment?: number;
	gameMode?: GameMode;
}

export interface MoveRequest {
	x: number;
	y: number;
}

export interface GameCreatedResponse {
	gameId: string;
	state: GameState;
}
