/**
 * boardUtils Tests
 *
 * Test-as-documentation: These tests define board utility behavior
 * One-at-a-time: Each test covers one specific behavior
 * Regression-proof: Tests verify coordinate calculations
 */

import { describe, it, expect } from 'vitest';
import { calculateGhostStonePosition, isValidCell, computeCellSize } from '$lib/utils/boardUtils';
import { GameConfig } from '$lib/config/gameConfig';

describe('boardUtils', () => {
  describe('calculateGhostStonePosition', () => {
    it('should offset ghost stone 50px above touch point by default', () => {
      const result = calculateGhostStonePosition(100, 200);
      expect(result.y).toBe(150);
      expect(result.x).toBe(100);
    });

    it('should accept custom offset', () => {
      const result = calculateGhostStonePosition(100, 200, 80);
      expect(result.y).toBe(120);
      expect(result.x).toBe(100);
    });

    it('should handle positive coordinates correctly', () => {
      const result = calculateGhostStonePosition(50, 100);
      expect(result.y).toBe(50);
      expect(result.x).toBe(50);
    });

    it('should handle negative coordinates correctly', () => {
      const result = calculateGhostStonePosition(-50, 100);
      expect(result.y).toBe(50);
      expect(result.x).toBe(-50);
    });

    it('should handle zero coordinates correctly', () => {
      const result = calculateGhostStonePosition(0, 50);
      expect(result.y).toBe(0);
      expect(result.x).toBe(0);
    });

    it('should handle large coordinates correctly', () => {
      const result = calculateGhostStonePosition(1000, 2000);
      expect(result.y).toBe(1950);
      expect(result.x).toBe(1000);
    });

    it('should handle fractional coordinates', () => {
      const result = calculateGhostStonePosition(10.5, 70.5);
      expect(result.y).toBe(20.5);
      expect(result.x).toBe(10.5);
    });

    it('should handle zero offset', () => {
      const result = calculateGhostStonePosition(100, 200, 0);
      expect(result.y).toBe(200);
      expect(result.x).toBe(100);
    });
  });

  describe('isValidCell', () => {
    it('should return true for valid coordinates', () => {
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(7, 7)).toBe(true);
      expect(isValidCell(15, 15)).toBe(true);
      expect(isValidCell(0, 15)).toBe(true);
      expect(isValidCell(15, 0)).toBe(true);
      expect(isValidCell(5, 10)).toBe(true);
    });

    it('should return false for out of bounds', () => {
      expect(isValidCell(-1, 0)).toBe(false);
      expect(isValidCell(0, -1)).toBe(false);
      expect(isValidCell(-1, -1)).toBe(false);
      expect(isValidCell(GameConfig.boardSize, 0)).toBe(false);
      expect(isValidCell(0, GameConfig.boardSize)).toBe(false);
      expect(isValidCell(GameConfig.boardSize, GameConfig.boardSize)).toBe(false);
      expect(isValidCell(20, 5)).toBe(false);
      expect(isValidCell(5, 20)).toBe(false);
    });

    it('should handle edge cases correctly', () => {
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(15, 15)).toBe(true);
      expect(isValidCell(0, GameConfig.boardSize)).toBe(false);
      expect(isValidCell(GameConfig.boardSize, 0)).toBe(false);
      expect(isValidCell(-0.1, 0)).toBe(false);
      expect(isValidCell(0, -0.1)).toBe(false);
      expect(isValidCell(15.1, 15)).toBe(true);
      expect(isValidCell(15, 15.1)).toBe(true);
    });

    it('should handle floating point numbers correctly', () => {
      expect(isValidCell(0.5, 0.5)).toBe(true);
      expect(isValidCell(7.3, 12.7)).toBe(true);
      expect(isValidCell(15.9, 15.9)).toBe(true);
      expect(isValidCell(-0.5, 0)).toBe(false);
      expect(isValidCell(0, -0.5)).toBe(false);
      expect(isValidCell(GameConfig.boardSize + 0.1, 0)).toBe(false);
      expect(isValidCell(0, GameConfig.boardSize + 0.1)).toBe(false);
    });

    it('should handle large numbers correctly', () => {
      expect(isValidCell(1000, 1000)).toBe(false);
      expect(isValidCell(999999, 999999)).toBe(false);
    });

    it('should handle zero correctly', () => {
      expect(isValidCell(0, 0)).toBe(true);
      expect(isValidCell(0, 15)).toBe(true);
      expect(isValidCell(15, 0)).toBe(true);
    });
  });

  describe('computeCellSize', () => {
    it('should compute cell size for a typical mobile viewport', () => {
      const size = computeCellSize(375);
      expect(size).toBe(Math.floor((375 * 0.95) / 16));
    });

    it('should cap at maxCellSize on large viewports', () => {
      const size = computeCellSize(1920);
      expect(size).toBe(64);
    });

    it('should not go below minCellSize', () => {
      const size = computeCellSize(100);
      expect(size).toBe(18);
    });

    it('should return maxCellSize on wide viewports', () => {
      const size = computeCellSize(1200);
      expect(size).toBe(64);
    });
  });
});
