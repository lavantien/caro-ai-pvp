import { describe, it, expect, vi, beforeEach } from 'vitest';
import { vibrate, vibrateOnValidMove, vibrateOnInvalidMove } from '$lib/utils/haptics';

describe('haptics', () => {
  beforeEach(() => {
    vi.stubGlobal('navigator', {
      vibrate: vi.fn(() => true),
    });
  });

  describe('vibrate', () => {
    it('should call navigator.vibrate with a number', () => {
      vibrate(100);
      expect(navigator.vibrate).toHaveBeenCalledWith(100);
    });

    it('should call navigator.vibrate with a pattern array', () => {
      vibrate([50, 100, 50]);
      expect(navigator.vibrate).toHaveBeenCalledWith([50, 100, 50]);
    });

    it('should not throw when navigator.vibrate is unavailable', () => {
      vi.stubGlobal('navigator', {});
      expect(() => vibrate(100)).not.toThrow();
    });
  });

  describe('vibrateOnValidMove', () => {
    it('should vibrate with short duration', () => {
      vibrateOnValidMove();
      expect(navigator.vibrate).toHaveBeenCalledWith(10);
    });
  });

  describe('vibrateOnInvalidMove', () => {
    it('should vibrate with error pattern', () => {
      vibrateOnInvalidMove();
      expect(navigator.vibrate).toHaveBeenCalledWith([30, 50, 30]);
    });
  });
});
