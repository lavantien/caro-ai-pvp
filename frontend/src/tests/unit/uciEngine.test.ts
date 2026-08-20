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

	sent: string[] = [];

	constructor(url: string) {
		this.url = url;
	}

	send = vi.fn((data: string) => {
		this.sent.push(data);
	});

	close = vi.fn(() => {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	});

	simulateOpen() {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.();
	}

	simulateLine(line: string) {
		this.onmessage?.({ data: line });
	}

	simulateError() {
		this.onerror?.(new Event('error'));
	}

	simulateClose() {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.();
	}
}

// The wire protocol is plaintext UCI lines, matching the backend handler.
describe('UCIEngine', () => {
	let engine: UCIEngine;
	let mockWs: MockWebSocket;

	beforeEach(() => {
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

	describe('initialize', () => {
		it('sends uci and resolves on uciok', async () => {
			await engine.connect();
			const promise = engine.initialize();
			mockWs.simulateLine('id name Caro AI');
			mockWs.simulateLine('uciok');
			const response = await promise;
			expect(response.uciOk).toBe(true);
			expect(mockWs.sent).toContain('uci');
		});

		it('ignores intermediate lines until uciok', async () => {
			await engine.connect();
			const promise = engine.initialize();
			mockWs.simulateLine('option name Threads type spin default 4');
			mockWs.simulateLine('uciok');
			await expect(promise).resolves.toEqual({ uciOk: true });
		});
	});

	describe('isReady', () => {
		it('sends isready and resolves true on readyok', async () => {
			await engine.connect();
			const promise = engine.isReady();
			mockWs.simulateLine('readyok');
			await expect(promise).resolves.toBe(true);
			expect(mockWs.sent).toContain('isready');
		});
	});

	describe('setPosition', () => {
		it('sends plaintext position command', async () => {
			await engine.connect();
			await engine.setPosition('startpos', ['hh', 'ih']);
			expect(mockWs.sent).toContain('position startpos moves hh ih');
		});
	});

	describe('getBestMoveAsync', () => {
		it('sends go with clocks and resolves on bestmove', async () => {
			await engine.connect();
			const promise = engine.getBestMoveAsync(['hh']);
			await Promise.resolve(); // let the awaited position send land
			expect(mockWs.sent).toContain('position startpos moves hh');
			expect(mockWs.sent).toContain('go wtime 180000 btime 180000 winc 2000 binc 2000');
			mockWs.simulateLine('info depth 3 nodes 1000 nps 100000 score cp 10 tt-hitrate 0.10 threads 1');
			mockWs.simulateLine('bestmove jj');
			await expect(promise).resolves.toBe('jj');
		});

		it('rejects on timeout', async () => {
			vi.useFakeTimers();
			await engine.connect();
			const promise = engine.getBestMoveAsync([]);
			promise.catch(() => {});
			await vi.advanceTimersByTimeAsync(60001);
			await expect(promise).rejects.toThrow('Search timeout');
			vi.useRealTimers();
		});

		it('rejects pending waits when the connection closes', async () => {
			await engine.connect();
			const promise = engine.getBestMoveAsync([]);
			promise.catch(() => {});
			await Promise.resolve(); // wait registration is async
			mockWs.simulateClose();
			await expect(promise).rejects.toThrow('connection closed');
		});
	});

	describe('setOption', () => {
		it('sends plaintext setoption command', async () => {
			await engine.connect();
			await engine.setOption('Threads', 4);
			expect(mockWs.sent).toContain('setoption name Threads value 4');
		});
	});

	describe('stop', () => {
		it('sends stop command', async () => {
			await engine.connect();
			const response = await engine.stop();
			expect(mockWs.sent).toContain('stop');
			expect(response.stopped).toBe(true);
		});
	});

	describe('error handling', () => {
		it('throws when sending a command while disconnected', async () => {
			await expect(engine.initialize()).rejects.toThrow('Not connected to UCI engine');
		});

		it('ignores unmatched lines', async () => {
			await engine.connect();
			expect(() => {
				mockWs.simulateLine('info depth 1 nodes 10');
			}).not.toThrow();
		});
	});
});
