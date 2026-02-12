/**
 * Centralized game constants - single source of truth for game rules.
 * Mirror of backend GameConstants.cs for frontend consistency.
 */

export const GameConfig = {
	/** Board size (32x32 grid) */
	boardSize: 32,

	/** Total number of cells on the board (32 * 32 = 1024) */
	totalCells: 1024,

	/** Center position index (16 is center of 0-31 range) */
	centerPosition: 16,

	/** Number of consecutive stones required to win */
	winLength: 5,

	/** ELO rating system K-factor */
	eloKFactor: 32,

	/** Default ELO rating for new players */
	defaultEloRating: 1500
} as const;

/**
 * AI evaluation scoring constants
 */
export const EvaluationConfig = {
	/** Score for five stones in a row (winning position) */
	fiveInRowScore: 100_000,

	/** Score for an open four */
	openFourScore: 10_000,

	/** Score for a closed four */
	closedFourScore: 1_000,

	/** Score for an open three */
	openThreeScore: 1_000,

	/** Score for a closed three */
	closedThreeScore: 100,

	/** Score for an open two */
	openTwoScore: 100,

	/** Bonus score for center control */
	centerBonus: 50
} as const;

/**
 * Network configuration for API endpoints
 */
export const NetworkConfig = {
	/** Base URL for API calls */
	apiBase: 'http://localhost:5207/api/tournament',

	/** URL for SignalR hub */
	hubUrl: 'http://localhost:5207/hubs/tournament'
} as const;
