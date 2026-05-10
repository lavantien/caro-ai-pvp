/**
 * GameStore Tests
 *
 * Test-as-documentation: These tests define game store behavior
 * One-at-a-time: Each test covers one specific behavior
 * Regression-proof: Tests verify state changes
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { GameStore } from './gameStore.svelte';

const mockEngine = {
	connect: vi.fn<() => Promise<boolean>>(),
	disconnect: vi.fn(),
	initialize: vi.fn<() => Promise<object>>(),
	getBestMoveAsync: vi.fn<() => Promise<string>>()
};

vi.mock('$lib/uciEngine', () => ({
	UCIEngine: vi.fn(function (this: Record<string, unknown>) {
		return Object.assign(this, mockEngine);
	}),
	movesToUCI: vi.fn((history: Array<{ x: number; y: number }>) =>
		history.map(m => `${String.fromCharCode(97 + m.x)}${m.y + 1}`)
	),
	fromUCI: vi.fn((move: string) => ({
		x: move.charCodeAt(0) - 97,
		y: parseInt(move.substring(1)) - 1
	})),
	toUCI: vi.fn()
}));

describe('GameStore', () => {
	let store: GameStore;

	beforeEach(() => {
		vi.clearAllMocks();
		mockEngine.connect.mockResolvedValue(true);
		mockEngine.initialize.mockResolvedValue({ uciOk: true });
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

	describe('UCI Integration', () => {
		it('connects to UCI engine successfully', async () => {
			const result = await store.connectUCI('ws://localhost:5207/ws/uci');
			expect(result).toBe(true);
			expect(store.uciConnected).toBe(true);
		});

		it('handles UCI connection failure', async () => {
			mockEngine.connect.mockRejectedValue(new Error('Connection failed'));
			const result = await store.connectUCI('ws://localhost:5207/ws/uci');
			expect(result).toBe(false);
			expect(store.uciConnected).toBe(false);
		});

		it('replaces existing engine on reconnect', async () => {
			await store.connectUCI('ws://localhost:5207/ws/uci');
			await store.connectUCI('ws://localhost:9999/ws/uci');
			expect(mockEngine.disconnect).toHaveBeenCalled();
		});

		it('disconnects from UCI engine', async () => {
			await store.connectUCI('ws://localhost:5207/ws/uci');
			store.disconnectUCI();
			expect(mockEngine.disconnect).toHaveBeenCalled();
			expect(store.uciConnected).toBe(false);
			expect(store.uciEngine).toBeNull();
		});

		it('disconnectUCI is no-op without engine', () => {
			store.disconnectUCI();
			expect(store.uciConnected).toBe(false);
		});

		it('returns null for AI move when disconnected', async () => {
			const result = await store.getAIMoveUCI();
			expect(result).toBeNull();
		});

		it('gets AI move via UCI engine', async () => {
			mockEngine.getBestMoveAsync.mockResolvedValue('h8');
			await store.connectUCI('ws://localhost:5207/ws/uci');
			store.makeMove(7, 7);
			const move = await store.getAIMoveUCI();
			expect(move).toEqual({ x: 7, y: 7 });
		});

		it('returns null for out-of-bounds UCI move', async () => {
			mockEngine.getBestMoveAsync.mockResolvedValue('q20');
			await store.connectUCI('ws://localhost:5207/ws/uci');
			const move = await store.getAIMoveUCI();
			expect(move).toBeNull();
		});

		it('returns null when getAIMoveUCI throws', async () => {
			mockEngine.getBestMoveAsync.mockRejectedValue(new Error('Engine error'));
			await store.connectUCI('ws://localhost:5207/ws/uci');
			const move = await store.getAIMoveUCI();
			expect(move).toBeNull();
		});

		it('enables UCI and auto-connects', async () => {
			expect(store.useUCI).toBe(false);
			store.setUseUCI(true);
			expect(store.useUCI).toBe(true);
		});

		it('disables UCI without disconnecting', () => {
			store.setUseUCI(false);
			expect(store.useUCI).toBe(false);
		});
	});
});
