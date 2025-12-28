import { describe, it, expect } from 'vitest';
import { calculateGhostStonePosition, isValidCell } from '$lib/utils/boardUtils';

describe('boardUtils', () => {
	describe('calculateGhostStonePosition', () => {
		it('should offset ghost stone 50px above touch point', () => {
			const result = calculateGhostStonePosition(100, 200);
			expect(result.y).toBe(150); // 200 - 50
			expect(result.x).toBe(100);  // No change
		});

		it('should offset negative coordinates correctly', () => {
			const result = calculateGhostStonePosition(-50, 100);
			expect(result.y).toBe(50);   // 100 - 50
			expect(result.x).toBe(-50);
		});
	});

	describe('isValidCell', () => {
		it('should return true for valid coordinates', () => {
			expect(isValidCell(0, 0)).toBe(true);
			expect(isValidCell(7, 7)).toBe(true);
			expect(isValidCell(14, 14)).toBe(true);
		});

		it('should return false for out of bounds', () => {
			expect(isValidCell(-1, 0)).toBe(false);
			expect(isValidCell(0, -1)).toBe(false);
			expect(isValidCell(15, 0)).toBe(false);
			expect(isValidCell(0, 15)).toBe(false);
		});
	});
});
