/**
 * Centralized game constants - single source of truth for game rules.
 * Mirror of backend Caro.Domain/Constants.cs for frontend consistency.
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

