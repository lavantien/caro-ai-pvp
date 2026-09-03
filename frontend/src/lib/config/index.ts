/**
 * Central configuration index - the single source of frontend constants.
 * Game-rule values mirror backend Caro.Domain/Constants.cs; the time-control
 * table mirrors backend Constants.Tables (minus the legacy Go aliases).
 * This module imports nothing from $lib/types: the value-domain unions live
 * here and $lib/types/game re-exports them.
 */

// --- Value-domain unions (re-exported by $lib/types/game) ---

export type GameMode = 'pvp' | 'pvai' | 'aivai';

export type DifficultyLevel = 1 | 2 | 3 | 4 | 5;

export interface TimeControlOption {
	value: string;
	label: string;
	initialTimeMs: number;
	incrementSeconds: number;
}

// --- API ---

// Optional chaining keeps the module importable outside Vite (Playwright
// loads the e2e timing section in plain Node, where import.meta.env is
// undefined).
const API_BASE_URL = import.meta.env?.VITE_API_BASE_URL || 'http://localhost:5207';
const WS_BASE_URL = API_BASE_URL.replace(/^http/, 'ws');

export const ApiConfig = {
	baseUrl: API_BASE_URL,
	wsBaseUrl: WS_BASE_URL,

	endpoints: {
		newGame: '/api/game/new',
		game: (id: string) => `/api/game/${id}`,
		move: (id: string) => `/api/game/${id}/move`,
		aiMove: (id: string) => `/api/game/${id}/ai-move`,
		undo: (id: string) => `/api/game/${id}/undo`
	} as const,

	wsEndpoints: {
		uci: '/ws/uci'
	} as const
} as const;

// --- Game rules ---

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

// --- Time controls ---

export const TIME_CONTROLS = [
	{ value: '1+0', label: '1+0 Bullet', initialTimeMs: 60_000, incrementSeconds: 0 },
	{ value: '3+0', label: '3+0 Blitz', initialTimeMs: 180_000, incrementSeconds: 0 },
	{ value: '3+2', label: '3+2 Blitz', initialTimeMs: 180_000, incrementSeconds: 2 },
	{ value: '7+5', label: '7+5 Rapid', initialTimeMs: 420_000, incrementSeconds: 5 },
	{ value: '10+0', label: '10+0 Rapid', initialTimeMs: 600_000, incrementSeconds: 0 },
	{ value: '15+10', label: '15+10 Classical', initialTimeMs: 900_000, incrementSeconds: 10 }
] as const satisfies readonly TimeControlOption[];

export type TimeControl = (typeof TIME_CONTROLS)[number]['value'];

/** Time control preselected for a new game */
export const DEFAULT_TIME_CONTROL: TimeControl = '7+5';

export function timeControlOption(value: string): Readonly<TimeControlOption> | undefined {
	return TIME_CONTROLS.find((tc) => tc.value === value);
}

export function timeControlLabel(value: string): string {
	return timeControlOption(value)?.label ?? value;
}

// --- Rating and leaderboard ---

export const RatingConfig = {
	/** localStorage key for persisting rating data */
	storageKey: 'caro-ratings',

	/** ELO expected score scale factor */
	eloScaleFactor: 400,

	/** Maximum players shown on leaderboard */
	topPlayersLimit: 10
} as const;

// --- UI ---

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
	lowTimeThresholdSeconds: 60,

	/** App content max width in pixels */
	maxContentWidthPx: 1024,

	/** Viewport width assumed for SSR cell sizing before the client measures */
	ssrViewportFallbackWidthPx: 1024,

	/** Coordinate label size as a fraction of cell size */
	labelSizeFraction: 0.55,

	/** Minimum coordinate label size in pixels */
	labelMinSizePx: 14,

	/** Ghost stone diameter as a fraction of cell size */
	ghostStoneScale: 0.78,

	/** Error banner auto-dismiss delay in milliseconds */
	errorMessageDismissMs: 5000
} as const;

// --- Audio ---

export const AudioConfig = {
	/** Master volume level (0.0 - 1.0) */
	volume: 0.5,

	/** Gain floor exponential ramps decay to (0 is inaudible but invalid) */
	envelopeFloor: 0.01,

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

// --- Haptics ---

export const HapticConfig = {
	/** Duration in ms for valid move vibration */
	validMoveDuration: 10,

	/** Vibration pattern for invalid move (vibrate-pause-vibrate) */
	invalidMovePattern: [30, 50, 30] as readonly number[]
} as const;

// --- E2E test timing ---

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
	regressionMoveWaitMs: 150,

	/** Window key exposing the current game id for e2e cleanup */
	gameIdHookKey: '__caroGameId'
} as const;

// --- UCI protocol ---
// Engine wire format is double-letter notation: letter for the row (y)
// followed by a letter for the column (x), e.g. "hh" is the center.

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
