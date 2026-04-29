import { describe, it, expect } from 'vitest';
import { toUCI, fromUCI } from '$lib/uciEngine';
import { GameConfig } from '$lib/config/gameConfig';

describe('UCI Coordinate Conversion', () => {
  describe('toUCI', () => {
    it('should convert origin (0,0) to a1', () => {
      expect(toUCI(0, 0)).toBe('a1');
    });

    it('should convert max column (15,0) to p1', () => {
      expect(toUCI(15, 0)).toBe('p1');
    });

    it('should convert max row (0,15) to a16', () => {
      expect(toUCI(0, 15)).toBe('a16');
    });

    it('should convert corner (15,15) to p16', () => {
      expect(toUCI(15, 15)).toBe('p16');
    });

    it('should convert (7,7) to h8', () => {
      expect(toUCI(7, 7)).toBe('h8');
    });

    it('should use single letter a-p for columns', () => {
      expect(toUCI(0, 0)).toMatch(/^a/);
      expect(toUCI(7, 0)).toMatch(/^h/);
      expect(toUCI(15, 0)).toMatch(/^p/);
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
    it('should convert a1 to (0,0)', () => {
      expect(fromUCI('a1')).toEqual({ x: 0, y: 0 });
    });

    it('should convert p1 to (15,0)', () => {
      expect(fromUCI('p1')).toEqual({ x: 15, y: 0 });
    });

    it('should convert a16 to (0,15)', () => {
      expect(fromUCI('a16')).toEqual({ x: 0, y: 15 });
    });

    it('should convert p16 to (15,15)', () => {
      expect(fromUCI('p16')).toEqual({ x: 15, y: 15 });
    });

    it('should convert h8 to (7,7)', () => {
      expect(fromUCI('h8')).toEqual({ x: 7, y: 7 });
    });

    it('should be case-insensitive', () => {
      expect(fromUCI('A1')).toEqual({ x: 0, y: 0 });
      expect(fromUCI('P16')).toEqual({ x: 15, y: 15 });
    });

    it('should throw for invalid column letters', () => {
      expect(() => fromUCI('q1')).toThrow();
      expect(() => fromUCI('z1')).toThrow();
    });

    it('should throw for invalid row numbers', () => {
      expect(() => fromUCI('a0')).toThrow();
      expect(() => fromUCI('a17')).toThrow();
    });

    it('should throw for too-short strings', () => {
      expect(() => fromUCI('')).toThrow();
      expect(() => fromUCI('a')).toThrow();
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
        toUCI(7, 7),
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
