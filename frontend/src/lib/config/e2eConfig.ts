/**
 * Centralized E2E test timing configuration.
 */

export const E2EConfig = {
	/** Wait after a standard move (ms) */
	moveWaitMs: 100,

	/** Wait after a move requiring API round-trip (ms) */
	apiMoveWaitMs: 200,

	/** Wait for win detection to process (ms) */
	winDetectionWaitMs: 1000,

	/** Wait for animation to complete (ms) */
	animationWaitMs: 600,

	/** Wait for timer countdown to be observable (ms) */
	timerCountdownWaitMs: 2000,

	/** Wait for regression test moves (ms) */
	regressionMoveWaitMs: 150
} as const;
