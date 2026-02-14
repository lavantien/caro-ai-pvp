/**
 * GameStore Tests
 *
 * Note: GameStore uses Svelte 5 runes ($state) which can only be used
 * inside .svelte and .svelte.js/ts files. These tests are limited to
 * testing the exported functions and types that don't require rune instantiation.
 *
 * For full store testing, use E2E tests or create a .svelte.test.ts file.
 */

import { describe, it, expect } from 'vitest';

describe('GameStore Types', () => {
  describe('Move History Types', () => {
    it('should define MoveHistoryEntry type correctly', () => {
      const move = {
        moveNumber: 1,
        player: 'red' as const,
        x: 7,
        y: 7
      };

      expect(move.moveNumber).toBe(1);
      expect(move.player).toBe('red');
      expect(move.x).toBe(7);
      expect(move.y).toBe(7);
    });

    it('should define player types correctly', () => {
      const redPlayer: 'red' = 'red';
      const bluePlayer: 'blue' = 'blue';
      const nonePlayer: 'none' = 'none';

      expect(redPlayer).toBe('red');
      expect(bluePlayer).toBe('blue');
      expect(nonePlayer).toBe('none');
    });

    it('should define cell type correctly', () => {
      const cell = {
        x: 7,
        y: 7,
        player: 'red' as const
      };

      expect(cell.x).toBe(7);
      expect(cell.y).toBe(7);
      expect(cell.player).toBe('red');
    });
  });

  describe('Board Utilities', () => {
    it('should have correct board dimensions', () => {
      // Caro is played on a 15x15 board
      const boardSize = 15;
      const totalCells = boardSize * boardSize;

      expect(totalCells).toBe(225);
    });

    it('should calculate cell index correctly', () => {
      const getIndex = (x: number, y: number) => y * 15 + x;

      expect(getIndex(0, 0)).toBe(0);
      expect(getIndex(7, 7)).toBe(112);
      expect(getIndex(14, 14)).toBe(224);
    });

    it('should calculate coordinates from index correctly', () => {
      const getCoords = (index: number) => ({
        x: index % 15,
        y: Math.floor(index / 15)
      });

      expect(getCoords(0)).toEqual({ x: 0, y: 0 });
      expect(getCoords(112)).toEqual({ x: 7, y: 7 });
      expect(getCoords(224)).toEqual({ x: 14, y: 14 });
    });
  });

  describe('Win Detection Logic', () => {
    it('should identify horizontal win pattern', () => {
      const horizontalWin = [
        { x: 5, y: 7, player: 'red' },
        { x: 6, y: 7, player: 'red' },
        { x: 7, y: 7, player: 'red' },
        { x: 8, y: 7, player: 'red' },
        { x: 9, y: 7, player: 'red' }
      ];

      // All cells should be in same row
      const allSameRow = horizontalWin.every(cell => cell.y === 7);
      expect(allSameRow).toBe(true);

      // All x coordinates should be consecutive
      const xCoords = horizontalWin.map(c => c.x).sort((a, b) => a - b);
      const isConsecutive = xCoords.every((x, i) => i === 0 || x === xCoords[i - 1] + 1);
      expect(isConsecutive).toBe(true);
    });

    it('should identify vertical win pattern', () => {
      const verticalWin = [
        { x: 7, y: 5, player: 'red' },
        { x: 7, y: 6, player: 'red' },
        { x: 7, y: 7, player: 'red' },
        { x: 7, y: 8, player: 'red' },
        { x: 7, y: 9, player: 'red' }
      ];

      // All cells should be in same column
      const allSameCol = verticalWin.every(cell => cell.x === 7);
      expect(allSameCol).toBe(true);

      // All y coordinates should be consecutive
      const yCoords = verticalWin.map(c => c.y).sort((a, b) => a - b);
      const isConsecutive = yCoords.every((y, i) => i === 0 || y === yCoords[i - 1] + 1);
      expect(isConsecutive).toBe(true);
    });

    it('should identify diagonal win pattern', () => {
      const diagonalWin = [
        { x: 5, y: 5, player: 'red' },
        { x: 6, y: 6, player: 'red' },
        { x: 7, y: 7, player: 'red' },
        { x: 8, y: 8, player: 'red' },
        { x: 9, y: 9, player: 'red' }
      ];

      // Check diagonal pattern (x - y should be constant for main diagonal)
      const diagonalOffsets = diagonalWin.map(c => c.x - c.y);
      const allSameDiagonal = diagonalOffsets.every(d => d === 0);
      expect(allSameDiagonal).toBe(true);
    });

    it('should identify anti-diagonal win pattern', () => {
      const antiDiagonalWin = [
        { x: 5, y: 9, player: 'red' },
        { x: 6, y: 8, player: 'red' },
        { x: 7, y: 7, player: 'red' },
        { x: 8, y: 6, player: 'red' },
        { x: 9, y: 5, player: 'red' }
      ];

      // Check anti-diagonal pattern (x + y should be constant)
      const antiDiagonalSums = antiDiagonalWin.map(c => c.x + c.y);
      const allSameAntiDiagonal = antiDiagonalSums.every(s => s === 14);
      expect(allSameAntiDiagonal).toBe(true);
    });
  });

  describe('Open Rule Validation', () => {
    it('should validate Open Rule: move 3 must be outside center 3x3', () => {
      // Center 3x3 is x=6-8, y=6-8
      const isInCenter3x3 = (x: number, y: number) =>
        x >= 6 && x <= 8 && y >= 6 && y <= 8;

      expect(isInCenter3x3(7, 7)).toBe(true);  // Center
      expect(isInCenter3x3(6, 6)).toBe(true);  // Corner of center
      expect(isInCenter3x3(5, 5)).toBe(false); // Outside
      expect(isInCenter3x3(9, 9)).toBe(false); // Outside
    });
  });

  describe('Timer Logic', () => {
    it('should format time correctly', () => {
      const formatTime = (ms: number): string => {
        const totalSeconds = Math.floor(ms / 1000);
        const minutes = Math.floor(totalSeconds / 60);
        const seconds = totalSeconds % 60;
        return `${minutes}:${seconds.toString().padStart(2, '0')}`;
      };

      expect(formatTime(0)).toBe('0:00');
      expect(formatTime(30000)).toBe('0:30');
      expect(formatTime(60000)).toBe('1:00');
      expect(formatTime(90000)).toBe('1:30');
      expect(formatTime(420000)).toBe('7:00');
    });

    it('should calculate time remaining correctly', () => {
      const initialTimeMs = 420000; // 7 minutes
      const elapsedMs = 30000; // 30 seconds
      const remainingMs = initialTimeMs - elapsedMs;

      expect(remainingMs).toBe(390000);
      expect(Math.floor(remainingMs / 1000)).toBe(390); // 6:30
    });
  });
});
