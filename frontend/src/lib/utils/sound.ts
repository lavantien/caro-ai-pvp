/**
 * Sound Manager
 *
 * Manages game sound effects with mute toggle and browser autoplay policy compliance.
 * Uses synthesized sounds via Web Audio API to avoid external asset dependencies.
 */

export class SoundManager {
	private muted: boolean = true; // Muted by default (browser autoplay policy)
	private audioContext?: AudioContext;
	private readonly volume: number = 0.5;

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
		const frequency = player === 'red' ? 440 : 523.25; // A4 vs C5
		oscillator.frequency.value = frequency;
		oscillator.type = 'sine';

		// Volume envelope (short, pleasant click)
		gainNode.gain.setValueAtTime(this.volume, this.audioContext.currentTime);
		gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + 0.1);

		// Connect and play
		oscillator.connect(gainNode);
		gainNode.connect(this.audioContext.destination);

		oscillator.start();
		oscillator.stop(this.audioContext.currentTime + 0.1);
	}

	/**
	 * Play victory sound (ascending arpeggio)
	 */
	playWinSound(winner: 'red' | 'blue'): void {
		if (this.muted) return;

		this.initAudio();
		if (!this.audioContext) return;

		// Create ascending arpeggio (C-E-G-C)
		const notes = winner === 'red' ? [523.25, 659.25, 783.99, 1046.5] : [659.25, 783.99, 987.77, 1318.51]; // C vs E arpeggio

		notes.forEach((freq, i) => {
			const oscillator = this.audioContext!.createOscillator();
			const gainNode = this.audioContext!.createGain();

			oscillator.frequency.value = freq;
			oscillator.type = 'sine';

			const startTime = this.audioContext!.currentTime + i * 0.1;
			const duration = 0.3;

			gainNode.gain.setValueAtTime(this.volume, startTime);
			gainNode.gain.exponentialRampToValueAtTime(0.01, startTime + duration);

			oscillator.connect(gainNode);
			gainNode.connect(this.audioContext!.destination);

			oscillator.start(startTime);
			oscillator.stop(startTime + duration);
		});
	}

	/**
	 * Internal helper to play HTML5 Audio elements (if using audio files)
	 * Currently unused (we use synthesized sounds instead)
	 */
	private playSound(audio: HTMLAudioElement): void {
		if (this.muted) {
			audio.volume = 0;
		} else {
			audio.volume = this.volume;
		}
	}
}

// Singleton instance
export const soundManager = new SoundManager();
