/**
 * boardUtils Tests
 *
 * Test-as-documentation: These tests define board utility behavior
 * One-at-a-time: Each test covers one specific behavior
 * Regression-proof: Tests verify coordinate calculations
 */

import { describe, it, expect } from 'vitest';
import { calculateGhostStonePosition, isValidCell } from '$lib/utils/boardUtils';

describe('boardUtils', () => {
  describe('calculateGhostStonePosition', () => {
    it('should offset ghost stone 50px above touch point', () => {
      const result = calculateGhostStonePosition(100, 200);
      expect(result.y).toBe(150); // 200 - 50
      expect(result.x).toBe(100);  // No change
    });

    it('should handle positive coordinates correctly', () => {
      const result = calculateGhostStonePosition(50, 100);
      expect(result.y).toBe(50);   // 100 - 50
      expect(result.x).toBe(50);
    });

    it('should handle negative coordinates correctly', () => {
      const result = calculateGhostStonePosition(-50, 100);
      expect(result.y).toBe(50);   // 100 - 50
      expect(result.x).toBe(-50);
    });

    it('should handle zero coordinates correctly', () => {
      const result = calculateGhostStonePosition(0, 50);
      expect(result.y).toBe(0);    // 50 - 50
      expect(result.x).toBe(0);
    });

    it('should handle large coordinates correctly', () => {
      const result = calculateGhostStonePosition(1000, 2000);
      expect(result.y).toBe(1950); // 2000 - 50
      expect(result.x).toBe(1000);
    });

    it('should handle fractional coordinates', () => {
      const result = calculateGhostStonePosition(10.5, 70.5);
      expect(result.y).toBe(20.5); // 70.5 - 50
      expect(result.x).toBe(10.5);
    });
  });

  describe('isValidCell', () => {
    it('should return true for valid coordinates', () => {
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(7, 7)).toBe(true);
      expect(isValidCell(14, 14)).toBe(true);
      expect(isValidCell(0, 14)).toBe(true);
      expect(isValidCell(14, 0)).toBe(true);
      expect(isValidCell(5, 10)).toBe(true);
    });

    it('should return false for out of bounds', () => {
      // Negative coordinates
      expect(isValidCell(-1, 0)).toBe(false);
      expect(isValidCell(0, -1)).toBe(false);
      expect(isValidCell(-1, -1)).toBe(false);

      // Coordinates beyond maximum (15 is the limit for 0-14 range)
      expect(isValidCell(15, 0)).toBe(false);
      expect(isValidCell(0, 15)).toBe(false);
      expect(isValidCell(15, 15)).toBe(false);
      expect(isValidCell(20, 5)).toBe(false);
      expect(isValidCell(5, 20)).toBe(false);
    });

    it('should handle edge cases correctly', () => {
      // Just inside bounds
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(14, 14)).toBe(true);

      // At bounds
      expect(isValidCell(0, 15)).toBe(false);
      expect(isValidCell(15, 0)).toBe(false);

      // Just outside bounds
      expect(isValidCell(-0.1, 0)).toBe(false);
      expect(isValidCell(0, -0.1)).toBe(false);
      expect(isValidCell(14.1, 14)).toBe(true); // 14.1 < 15
      expect(isValidCell(14, 14.1)).toBe(true); // 14.1 < 15
    });

    it('should handle floating point numbers correctly', () => {
      // Valid floating point coordinates within range
      expect(isValidCell(0.5, 0.5)).toBe(true);
      expect(isValidCell(7.3, 12.7)).toBe(true);
      expect(isValidCell(14.9, 14.9)).toBe(true);

      // Invalid floating point coordinates
      expect(isValidCell(-0.5, 0)).toBe(false);
      expect(isValidCell(0, -0.5)).toBe(false);
      expect(isValidCell(15.1, 0)).toBe(false);
      expect(isValidCell(0, 15.1)).toBe(false);
    });

    it('should handle large numbers correctly', () => {
      // Large valid coordinates
      expect(isValidCell(1000, 1000)).toBe(false); // Too large

      // Large invalid coordinates
      expect(isValidCell(999999, 999999)).toBe(false);
    });

    it('should handle zero correctly', () => {
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(0, 14)).toBe(true);
      expect(isValidCell(14, 0)).toBe(true);
    });
  });
});