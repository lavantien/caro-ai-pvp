/**
 * Centralized UI configuration - dimensions, colors, animation timing.
 */

export const UIConfig = {
	/** Maximum board cell size in pixels (desktop cap) */
	maxCellSize: 64,

	/** Minimum board cell size in pixels */
	minCellSize: 18,

	/** Fraction of viewport width the board occupies */
	boardWidthFraction: 0.95,

	/** Touch ghost stone vertical offset in pixels */
	ghostStoneTouchOffset: 50,

	/** Winning line SVG stroke width */
	winningLineStrokeWidth: 6,

	/** Winning line color (Tailwind red-500) */
	winningLineColor: '#ef4444',

	/** Winning line draw animation duration in milliseconds */
	winningLineAnimationMs: 500,

	/** Timer display update interval in milliseconds */
	timerUpdateIntervalMs: 100,

	/** Timer server sync interval in milliseconds */
	timerSyncIntervalMs: 500,

	/** Seconds remaining to trigger low-time warning */
	lowTimeThresholdSeconds: 60
} as const;
