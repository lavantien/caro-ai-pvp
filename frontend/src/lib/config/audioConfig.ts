/**
 * Centralized audio configuration - frequencies, volumes, durations.
 */

export const AudioConfig = {
	/** Master volume level (0.0 - 1.0) */
	volume: 0.5,

	frequencies: {
		/** Red stone placement tone (A4) */
		redStone: 440,
		/** Blue stone placement tone (C5) */
		blueStone: 523.25
	} as const,

	winArpeggios: {
		/** Red win ascending arpeggio (C5-E5-G5-C6) */
		red: [523.25, 659.25, 783.99, 1046.5] as readonly number[],
		/** Blue win ascending arpeggio (E5-G5-B5-E6) */
		blue: [659.25, 783.99, 987.77, 1318.51] as readonly number[]
	} as const,

	durations: {
		/** Sound envelope fade-out in seconds */
		envelope: 0.1,
		/** Individual win note duration in seconds */
		winNote: 0.3,
		/** Delay between arpeggio notes in seconds */
		noteDelay: 0.1
	} as const
} as const;
