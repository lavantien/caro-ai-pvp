/**
 * Sound Manager
 *
 * Manages game sound effects with mute toggle and browser autoplay policy compliance.
 * Uses synthesized sounds via Web Audio API to avoid external asset dependencies.
 */

import { AudioConfig } from '$lib/config/audioConfig';

export class SoundManager {
	private muted: boolean = true; // Muted by default (browser autoplay policy)
	private audioContext?: AudioContext;
	private readonly volume: number = AudioConfig.volume;

	constructor() {
		// Don't initialize AudioContext until user interaction (browser policy)
	}

	/**
	 * Check if sound is currently muted
	 */
	isMuted(): boolean {
		return this.muted;
	}

	/**
	 * Toggle mute state
	 */
	toggleMute(): void {
		this.muted = !this.muted;
		if (!this.muted) {
			this.initAudio();
		}
	}

	/**
	 * Initialize AudioContext (must be called after user interaction)
	 */
	private initAudio(): void {
		if (!this.audioContext) {
			this.audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
		}
	}

	/**
	 * Play stone placement sound (different tone for Red vs Blue)
	 */
	playStoneSound(player: 'red' | 'blue'): void {
		if (this.muted) return;

		this.initAudio();
		if (!this.audioContext) return;

		// Create oscillator for synthesized sound
		const oscillator = this.audioContext.createOscillator();
		const gainNode = this.audioContext.createGain();

		// Different tones for each player
		const frequency = player === 'red' ? AudioConfig.frequencies.redStone : AudioConfig.frequencies.blueStone;
		oscillator.frequency.value = frequency;
		oscillator.type = 'sine';

		// Volume envelope (short, pleasant click)
		gainNode.gain.setValueAtTime(this.volume, this.audioContext.currentTime);
		gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + AudioConfig.durations.envelope);

		// Connect and play
		oscillator.connect(gainNode);
		gainNode.connect(this.audioContext.destination);

		oscillator.start();
		oscillator.stop(this.audioContext.currentTime + AudioConfig.durations.envelope);
	}

	/**
	 * Play victory sound (ascending arpeggio)
	 */
	playWinSound(winner: 'red' | 'blue'): void {
		if (this.muted) return;

		this.initAudio();
		if (!this.audioContext) return;

		// Create ascending arpeggio
		const notes = winner === 'red' ? AudioConfig.winArpeggios.red : AudioConfig.winArpeggios.blue;

		notes.forEach((freq, i) => {
			const oscillator = this.audioContext!.createOscillator();
			const gainNode = this.audioContext!.createGain();

			oscillator.frequency.value = freq;
			oscillator.type = 'sine';

			const startTime = this.audioContext!.currentTime + i * AudioConfig.durations.noteDelay;
			const duration = AudioConfig.durations.winNote;

			gainNode.gain.setValueAtTime(this.volume, startTime);
			gainNode.gain.exponentialRampToValueAtTime(0.01, startTime + duration);

			oscillator.connect(gainNode);
			gainNode.connect(this.audioContext!.destination);

			oscillator.start(startTime);
			oscillator.stop(startTime + duration);
		});
	}

}

// Singleton instance
export const soundManager = new SoundManager();
