/**
 * GameStore Tests
 *
 * Test-as-documentation: These tests define game store behavior
 * One-at-a-time: Each test covers one specific behavior
 * Regression-proof: Tests verify state changes
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { GameStore } from './gameStore.svelte';

describe('GameStore', () => {
	let store: GameStore;

	beforeEach(() => {
		store = new GameStore();
	});

	describe('Move History Tracking', () => {
		it('should initialize with empty move history', () => {
			expect(store.moveHistory).toEqual([]);
		});

		it('should record move when makeMove is called', () => {
			store.makeMove(7, 7);

			expect(store.moveHistory).toHaveLength(1);
			expect(store.moveHistory[0]).toEqual({
				moveNumber: 1,
				player: 'red',
				x: 7,
				y: 7
			});
		});

		it('should record multiple moves in order', () => {
			store.makeMove(7, 7);
			store.makeMove(7, 8);
			store.makeMove(8, 8);

			expect(store.moveHistory).toHaveLength(3);
			expect(store.moveHistory[0]).toEqual({ moveNumber: 1, player: 'red', x: 7, y: 7 });
			expect(store.moveHistory[1]).toEqual({ moveNumber: 2, player: 'blue', x: 7, y: 8 });
			expect(store.moveHistory[2]).toEqual({ moveNumber: 3, player: 'red', x: 8, y: 8 });
		});

		it('should not record invalid moves', () => {
			store.makeMove(7, 7);
			store.makeMove(7, 7); // Same position, should fail

			expect(store.moveHistory).toHaveLength(1);
		});

		it('should clear move history on reset', () => {
			store.makeMove(7, 7);
			store.makeMove(7, 8);
			store.reset();

			expect(store.moveHistory).toEqual([]);
		});

		it('should track current move number correctly', () => {
			expect(store.moveNumber).toBe(0);

			store.makeMove(7, 7);
			expect(store.moveNumber).toBe(1);

			store.makeMove(7, 8);
			expect(store.moveNumber).toBe(2);
		});
	});
});
