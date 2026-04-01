import { describe, it, expect } from 'vitest';
import { toUCI, fromUCI } from '$lib/uciEngine';
import { GameConfig } from '$lib/config/gameConfig';

describe('UCI Coordinate Conversion', () => {
  describe('toUCI', () => {
    it('should convert origin (0,0) to aa1', () => {
      expect(toUCI(0, 0)).toBe('aa1');
    });

    it('should convert max column (15,0) to dd1', () => {
      expect(toUCI(15, 0)).toBe('dd1');
    });

    it('should convert max row (0,15) to aa16', () => {
      expect(toUCI(0, 15)).toBe('aa16');
    });

    it('should convert corner (15,15) to dd16', () => {
      expect(toUCI(15, 15)).toBe('dd16');
    });

    it('should convert (7,8) to bd9', () => {
      expect(toUCI(7, 8)).toBe('bd9');
    });

    it('should use a-d first letter for column groups', () => {
      // Group 0: x=0-3 -> a?
      expect(toUCI(0, 0)).toMatch(/^a/);
      expect(toUCI(3, 0)).toMatch(/^a/);
      // Group 1: x=4-7 -> b?
      expect(toUCI(4, 0)).toMatch(/^b/);
      expect(toUCI(7, 0)).toMatch(/^b/);
      // Group 2: x=8-11 -> c?
      expect(toUCI(8, 0)).toMatch(/^c/);
      expect(toUCI(11, 0)).toMatch(/^c/);
      // Group 3: x=12-15 -> d?
      expect(toUCI(12, 0)).toMatch(/^d/);
      expect(toUCI(15, 0)).toMatch(/^d/);
    });

    it('should throw for x out of bounds', () => {
      expect(() => toUCI(-1, 0)).toThrow();
      expect(() => toUCI(GameConfig.boardSize, 0)).toThrow();
    });

    it('should throw for y out of bounds', () => {
      expect(() => toUCI(0, -1)).toThrow();
      expect(() => toUCI(0, GameConfig.boardSize)).toThrow();
    });
  });

  describe('fromUCI', () => {
    it('should convert aa1 to (0,0)', () => {
      expect(fromUCI('aa1')).toEqual({ x: 0, y: 0 });
    });

    it('should convert dd1 to (15,0)', () => {
      expect(fromUCI('dd1')).toEqual({ x: 15, y: 0 });
    });

    it('should convert aa16 to (0,15)', () => {
      expect(fromUCI('aa16')).toEqual({ x: 0, y: 15 });
    });

    it('should convert dd16 to (15,15)', () => {
      expect(fromUCI('dd16')).toEqual({ x: 15, y: 15 });
    });

    it('should convert bd9 to (7,8)', () => {
      expect(fromUCI('bd9')).toEqual({ x: 7, y: 8 });
    });

    it('should be case-insensitive', () => {
      expect(fromUCI('AA1')).toEqual({ x: 0, y: 0 });
      expect(fromUCI('Dd16')).toEqual({ x: 15, y: 15 });
    });

    it('should throw for invalid column letters', () => {
      expect(() => fromUCI('ea1')).toThrow();
      expect(() => fromUCI('ae1')).toThrow();
    });

    it('should throw for invalid row numbers', () => {
      expect(() => fromUCI('aa0')).toThrow();
      expect(() => fromUCI('aa17')).toThrow();
    });

    it('should throw for too-short strings', () => {
      expect(() => fromUCI('')).toThrow();
      expect(() => fromUCI('a')).toThrow();
      expect(() => fromUCI('aa')).toThrow();
    });
  });

  describe('round-trip', () => {
    it(`should round-trip all ${GameConfig.totalCells} board positions`, () => {
      for (let x = 0; x < GameConfig.boardSize; x++) {
        for (let y = 0; y < GameConfig.boardSize; y++) {
          const uci = toUCI(x, y);
          const result = fromUCI(uci);
          expect(result).toEqual({ x, y });
        }
      }
    });

    it('should round-trip through string parsing', () => {
      const positions = [
        toUCI(0, 0),
        toUCI(15, 15),
        toUCI(7, 8), // bd9
        toUCI(3, 3),
        toUCI(12, 12),
      ];

      for (const uci of positions) {
        const { x, y } = fromUCI(uci);
        expect(toUCI(x, y)).toBe(uci);
      }
    });
  });
});
