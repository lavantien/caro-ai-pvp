import { GameConfig } from '$lib/config/gameConfig';

export interface Point {
	x: number;
	y: number;
}

export function calculateGhostStonePosition(x: number, y: number): Point {
	return { x, y: y - 50 };
}

export function isValidCell(x: number, y: number): boolean {
	return x >= 0 && x < GameConfig.boardSize && y >= 0 && y < GameConfig.boardSize;
}
