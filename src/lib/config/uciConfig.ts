import { ApiConfig } from './apiConfig';
import { GameConfig } from './gameConfig';

/**
 * Centralized UCI protocol configuration - coordinate system and defaults.
 */

export const UCIConfig = {
	/** Number of columns per letter group in double-letter notation */
	letterGroupSize: 4,

	/** ASCII code for lowercase 'a' (coordinate origin) */
	asciiLowerA: 97,

	/** Minimum valid row number (1-based) */
	minRow: 1,

	/** Maximum valid row number (1-based) */
	maxRow: GameConfig.boardSize,

	/** Minimum UCI move string length */
	minMoveLength: 3,

	/** Default time per player in milliseconds (3 minutes) */
	defaultTimeMs: 180000,

	/** Default increment per move in milliseconds (2 seconds) */
	defaultIncrementMs: 2000,

	/** Search timeout in milliseconds (60 seconds) */
	searchTimeoutMs: 60000,

	/** Default WebSocket URL for UCI engine */
	defaultWsUrl: `${ApiConfig.wsBaseUrl}${ApiConfig.wsEndpoints.uci}`
} as const;
