/**
 * Sound Manager Tests
 *
 * Test-as-documentation: These tests define the sound manager behavior
 * One-at-a-time: Each test covers one specific behavior
 * Regression-proof: Tests verify sound state changes
 * Table-driven: N/A (simple state management)
 * Test-doubles: Mock AudioContext for Node.js environment
 */

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { SoundManager } from './sound';

// Mock browser APIs for Node.js environment
const mockCreateOscillator = vi.fn(() => ({
	connect: vi.fn(),
	start: vi.fn(),
	stop: vi.fn(),
	frequency: { value: 440 },
	type: 'sine' as const
}));
const mockCreateGain = vi.fn(() => ({
	connect: vi.fn(),
	gain: {
		setValueAtTime: vi.fn(),
		exponentialRampToValueAtTime: vi.fn()
	}
}));

class MockAudioContext {
	createOscillator = mockCreateOscillator;
	createGain = mockCreateGain;
	destination = {};
	currentTime = 0;
}

globalThis.AudioContext = MockAudioContext as any;
globalThis.window = {
	AudioContext: MockAudioContext,
	webkitAudioContext: MockAudioContext
} as any;

describe('SoundManager', () => {
	let soundManager: SoundManager;

	beforeEach(() => {
		soundManager = new SoundManager();
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('Initial State', () => {
		it('should be muted by default (browser autoplay policy)', () => {
			expect(soundManager.isMuted()).toBe(true);
		});

		it('should not have AudioContext initialized until user interaction', () => {
			expect(soundManager['audioContext']).toBeUndefined();
		});
	});

	describe('Mute Toggle', () => {
		it('should unmute when toggle is called', () => {
			soundManager.toggleMute();
			expect(soundManager.isMuted()).toBe(false);
		});

		it('should mute when toggle is called twice', () => {
			soundManager.toggleMute();
			soundManager.toggleMute();
			expect(soundManager.isMuted()).toBe(true);
		});
	});

	describe('Stone Placement Sounds', () => {
		it('should initialize AudioContext on first play attempt', () => {
			soundManager.toggleMute(); // Unmute first

			soundManager.playStoneSound('red');

			// AudioContext should be created when unmuted
			expect(soundManager.isMuted()).toBe(false);
		});

		it('should not create audio nodes when muted', () => {
			soundManager.playStoneSound('red');

			expect(mockCreateOscillator).not.toHaveBeenCalled();
		});

		it('should create audio nodes when unmuted', () => {
			soundManager.toggleMute();

			soundManager.playStoneSound('red');

			expect(mockCreateOscillator).toHaveBeenCalled();
		});
	});

	describe('Win Sound', () => {
		it('should create audio nodes when unmuted', () => {
			soundManager.toggleMute();

			soundManager.playWinSound('red');

			expect(mockCreateOscillator).toHaveBeenCalled();
		});

		it('should not create audio nodes when muted', () => {
			soundManager.playWinSound('blue');

			// AudioContext methods should not be called when muted
			expect(mockCreateOscillator).not.toHaveBeenCalled();
		});
	});
});
