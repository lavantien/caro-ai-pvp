import { GameConfig } from '$lib/config/gameConfig';
import { UIConfig } from '$lib/config/uiConfig';

export interface Point {
	x: number;
	y: number;
}

export function calculateGhostStonePosition(x: number, y: number): Point {
	return { x, y: y - UIConfig.ghostStoneTouchOffset };
}

export function isValidCell(x: number, y: number): boolean {
	return x >= 0 && x < GameConfig.boardSize && y >= 0 && y < GameConfig.boardSize;
}
