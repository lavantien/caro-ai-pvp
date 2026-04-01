/**
 * Centralized haptic feedback configuration.
 */

export const HapticConfig = {
	/** Duration in ms for valid move vibration */
	validMoveDuration: 10,

	/** Vibration pattern for invalid move (vibrate-pause-vibrate) */
	invalidMovePattern: [30, 50, 30] as readonly number[]
} as const;
