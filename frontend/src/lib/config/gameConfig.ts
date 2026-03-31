/**
 * Centralized game constants - single source of truth for game rules.
 * Mirror of backend GameConstants.cs for frontend consistency.
 */

export const GameConfig = {
	/** Board size (16x16 grid) */
	boardSize: 16,

	/** Total number of cells on the board (16 * 16 = 256) */
	totalCells: 256,

	/** Center position index (8 is center of 0-15 range) */
	centerPosition: 8,

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

