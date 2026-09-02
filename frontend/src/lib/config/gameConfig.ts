/**
 * Centralized game constants - single source of truth for game rules.
 * Mirror of backend Caro.Domain/Constants.cs for frontend consistency.
 */

import type { DifficultyLevel, GameMode } from '$lib/types/game';
import type { TimeControl } from './timeControlConfig';

export const GameConfig = {
	/** Board size (16x16 grid) */
	boardSize: 16,

	/** Total number of cells on the board (16 * 16 = 256) */
	totalCells: 256,

	/** Center position index (8 is center of 0-15 range) */
	centerPosition: 8,

	/** Number of consecutive stones required to win */
	winLength: 5,

	/** Move number red's second stone is constrained by the open rule */
	openRuleSecondMoveNumber: 2,

	/** Minimum distance of red's second stone from the first */
	openRuleMinDistance: 3,

	/** Lowest selectable AI difficulty */
	minDifficulty: 1,

	/** Highest selectable AI difficulty */
	maxDifficulty: 5,

	/** ELO rating system K-factor */
	eloKFactor: 32,

	/** Default ELO rating for new players */
	defaultEloRating: 1500,

	/** Initial values for the game page setup controls */
	defaultGameSetup: {
		gameMode: 'pvp',
		timeControl: '7+5',
		aiSide: 'blue',
		difficulty: 5
	} as const satisfies {
		gameMode: GameMode;
		timeControl: TimeControl;
		aiSide: 'red' | 'blue';
		difficulty: DifficultyLevel;
	}
} as const;

