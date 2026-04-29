import { ApiConfig } from './apiConfig';
import { GameConfig } from './gameConfig';

/**
 * Centralized UCI protocol configuration - coordinate system and defaults.
 * Uses simple algebraic notation: columns a-p, rows 1-16.
 */

export const UCIConfig = {
	/** ASCII code for lowercase 'a' (coordinate origin) */
	asciiLowerA: 97,

	/** Minimum valid row number (1-based) */
	minRow: 1,

	/** Maximum valid row number (1-based) */
	maxRow: GameConfig.boardSize,

	/** Default time per player in milliseconds (3 minutes) */
	defaultTimeMs: 180000,

	/** Default increment per move in milliseconds (2 seconds) */
	defaultIncrementMs: 2000,

	/** Search timeout in milliseconds (60 seconds) */
	searchTimeoutMs: 60000,

	/** Default WebSocket URL for UCI engine */
	defaultWsUrl: `${ApiConfig.wsBaseUrl}${ApiConfig.wsEndpoints.uci}`
} as const;
