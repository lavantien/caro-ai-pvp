export interface Point {
	x: number;
	y: number;
}

export function calculateGhostStonePosition(x: number, y: number): Point {
	return { x, y: y - 50 };
}

export function isValidCell(x: number, y: number): boolean {
	return x >= 0 && x < 15 && y >= 0 && y < 15;
}
