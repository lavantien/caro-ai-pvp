import { describe, it, expect } from 'vitest';
import { ApiConfig } from '$lib/config/apiConfig';

describe('ApiConfig', () => {
	it('has baseUrl as a string starting with http', () => {
		expect(ApiConfig.baseUrl).toBeTruthy();
		expect(ApiConfig.baseUrl).toMatch(/^http/);
	});

	it('derives wsBaseUrl by replacing http with ws', () => {
		expect(ApiConfig.wsBaseUrl).toBe(
			ApiConfig.baseUrl.replace(/^http/, 'ws')
		);
	});

	it('has static newGame endpoint', () => {
		expect(ApiConfig.endpoints.newGame).toBe('/api/game/new');
	});

	it('has dynamic game endpoint', () => {
		expect(ApiConfig.endpoints.game('abc123')).toBe('/api/game/abc123');
	});

	it('has dynamic move endpoint', () => {
		expect(ApiConfig.endpoints.move('abc123')).toBe('/api/game/abc123/move');
	});

	it('has dynamic aiMove endpoint', () => {
		expect(ApiConfig.endpoints.aiMove('abc123')).toBe('/api/game/abc123/ai-move');
	});

	it('has dynamic undo endpoint', () => {
		expect(ApiConfig.endpoints.undo('abc123')).toBe('/api/game/abc123/undo');
	});

	it('has ws uci endpoint', () => {
		expect(ApiConfig.wsEndpoints.uci).toBe('/ws/uci');
	});
});
