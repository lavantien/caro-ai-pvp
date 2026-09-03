import { GameConfig, UIConfig, UCIConfig } from '$lib/config';

export interface Point {
	x: number;
	y: number;
}

/**
 * Display notation: column letter (a-p) from x plus 1-based row number from y,
 * e.g. (7, 7) -> "h8". Matches the board's coordinate labels; the UCI wire
 * format (toUCI) stays double-letter.
 */
export function toAlgebraic(x: number, y: number): string {
	if (x < 0 || x >= GameConfig.boardSize || y < 0 || y >= GameConfig.boardSize) {
		throw new Error(`Coordinates out of bounds: (${x}, ${y})`);
	}
	const columnLetter = String.fromCharCode(UCIConfig.asciiLowerA + x);
	return `${columnLetter}${y + 1}`;
}

export function calculateGhostStonePosition(x: number, y: number, offset: number = UIConfig.ghostStoneTouchOffset): Point {
	return { x, y: y - offset };
}

export function isValidCell(x: number, y: number): boolean {
	return (
		Number.isInteger(x) &&
		Number.isInteger(y) &&
		x >= 0 &&
		x < GameConfig.boardSize &&
		y >= 0 &&
		y < GameConfig.boardSize
	);
}

export function computeCellSize(viewportWidth: number): number {
	const size = Math.floor((viewportWidth * UIConfig.boardWidthFraction) / GameConfig.boardSize);
	return Math.max(UIConfig.minCellSize, Math.min(UIConfig.maxCellSize, size));
}
