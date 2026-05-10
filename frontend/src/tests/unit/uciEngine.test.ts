import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { UCIEngine } from '$lib/uciEngine';

class MockWebSocket {
	static OPEN = 1;
	static CLOSED = 3;

	url: string;
	readyState: number = MockWebSocket.OPEN;
	onopen: (() => void) | null = null;
	onmessage: ((event: { data: string }) => void) | null = null;
	onerror: ((error: Event) => void) | null = null;
	onclose: (() => void) | null = null;

	private responseQueue: object[] = [];

	constructor(url: string) {
		this.url = url;
	}

	send = vi.fn((data: string) => {
		const response = this.responseQueue.shift();
		if (response) {
			queueMicrotask(() => this.simulateMessage(response));
		}
	});

	close = vi.fn(() => {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	});

	enqueueResponse(...responses: object[]) {
		this.responseQueue.push(...responses);
	}

	simulateOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	simulateMessage(data: object) {
		this.onmessage?.({ data: JSON.stringify(data) });
	}

	simulateError() {
		this.onerror?.(new Event('error'));
	}

	simulateClose() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}
}

describe('UCIEngine', () => {
	let engine: UCIEngine;
	let mockWs: MockWebSocket;

	beforeEach(() => {
		vi.stubGlobal('$state', (v: unknown) => v);
		mockWs = new MockWebSocket('ws://localhost:5207/ws/uci');
		vi.stubGlobal('WebSocket', class extends MockWebSocket {
			constructor(url: string) {
				super(url);
				mockWs = this;
				queueMicrotask(() => this.simulateOpen());
			}
		});
		engine = new UCIEngine('ws://localhost:5207/ws/uci');
	});

	afterEach(() => {
		engine.disconnect();
		vi.unstubAllGlobals();
	});

	describe('constructor', () => {
		it('accepts custom URL', () => {
			const e = new UCIEngine('ws://custom:1234');
			expect(e).toBeDefined();
		});

		it('uses default URL when none provided', () => {
			const e = new UCIEngine();
			expect(e).toBeDefined();
		});
	});

	describe('connect', () => {
		it('connects successfully', async () => {
			const result = await engine.connect();
			expect(result).toBe(true);
			expect(engine.isConnected()).toBe(true);
		});

		it('returns true if already connected', async () => {
			await engine.connect();
			const result = await engine.connect();
			expect(result).toBe(true);
		});

		it('rejects on WebSocket error', async () => {
			vi.stubGlobal('WebSocket', class extends MockWebSocket {
				constructor(url: string) {
					super(url);
					mockWs = this;
					queueMicrotask(() => this.simulateError());
				}
			});
			const fresh = new UCIEngine('ws://localhost:5207/ws/uci');
			await expect(fresh.connect()).rejects.toThrow('WebSocket connection failed');
		});
	});

	describe('disconnect', () => {
		it('closes the WebSocket and clears reference', async () => {
			await engine.connect();
			engine.disconnect();
			expect(engine.isConnected()).toBe(false);
			expect(mockWs.close).toHaveBeenCalled();
		});

		it('is a no-op when not connected', () => {
			engine.disconnect();
			expect(engine.isConnected()).toBe(false);
		});
	});

	describe('isConnected', () => {
		it('returns false before connect', () => {
			expect(engine.isConnected()).toBe(false);
		});

		it('returns true after connect', async () => {
			await engine.connect();
			expect(engine.isConnected()).toBe(true);
		});
	});

	async function connectAndWait() {
		await engine.connect();
	}

	describe('initialize', () => {
		it('sends uci command and returns uciOk', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ uciOk: true, id: ['engine'] });
			const response = await engine.initialize();
			expect(response.uciOk).toBe(true);
		});
	});

	describe('isReady', () => {
		it('returns true when engine responds readyOk', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ readyOk: true });
			const result = await engine.isReady();
			expect(result).toBe(true);
		});

		it('returns false when response lacks readyOk', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			const result = await engine.isReady();
			expect(result).toBe(false);
		});
	});

	describe('newGame', () => {
		it('sends ucinewgame command', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			const response = await engine.newGame();
			expect(response.ok).toBe(true);
			expect(mockWs.send).toHaveBeenCalled();
		});
	});

	describe('setPosition', () => {
		it('sends position command with default startpos', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			await engine.setPosition();
			const sent = JSON.parse(mockWs.send.mock.calls[0][0] as string);
			expect(sent.command).toBe('position');
			expect(sent.position).toBe('startpos');
		});

		it('sends position command with moves', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			await engine.setPosition('startpos', ['h8', 'h9']);
			const sent = JSON.parse(mockWs.send.mock.calls[0][0] as string);
			expect(sent.moves).toEqual(['h8', 'h9']);
		});
	});

	describe('getBestMove', () => {
		it('returns best move from engine', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true }, { bestMove: 'i9' });
			const move = await engine.getBestMove(['h8']);
			expect(move).toBe('i9');
		});

		it('throws when no best move returned', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true }, { info: { depth: 1 } });
			await expect(engine.getBestMove([])).rejects.toThrow('No best move returned');
		});
	});

	describe('getBestMoveAsync', () => {
		it('resolves with best move via callback', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true }, { bestMove: 'j10' });
			const move = await engine.getBestMoveAsync(['h8']);
			expect(move).toBe('j10');
		});

		it('rejects on timeout', async () => {
			vi.useFakeTimers();
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			const promise = engine.getBestMoveAsync([]);
			promise.catch(() => {});
			await vi.advanceTimersByTimeAsync(60001);
			await expect(promise).rejects.toThrow('Search timeout');
			vi.useRealTimers();
		});
	});

	describe('setOption', () => {
		it('sends setoption command with stringified value', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ ok: true });
			await engine.setOption('Threads', 4);
			const sent = JSON.parse(mockWs.send.mock.calls[0][0] as string);
			expect(sent.command).toBe('setoption');
			expect(sent.name).toBe('Threads');
			expect(sent.value).toBe('4');
		});
	});

	describe('stop', () => {
		it('sends stop command', async () => {
			await connectAndWait();
			mockWs.enqueueResponse({ stopped: true });
			const response = await engine.stop();
			expect(response.stopped).toBe(true);
		});
	});

	describe('error handling', () => {
		it('throws when sending command while disconnected', async () => {
			await expect(engine.initialize()).rejects.toThrow('Not connected to UCI engine');
		});

		it('handles invalid JSON in handleMessage gracefully', async () => {
			await connectAndWait();
			expect(() => {
				mockWs.onmessage?.({ data: 'not-json' });
			}).not.toThrow();
		});
	});
});
