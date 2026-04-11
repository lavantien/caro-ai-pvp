/**
 * Centralized rating/leaderboard configuration.
 */

export const RatingConfig = {
	/** localStorage key for persisting rating data */
	storageKey: 'caro-ratings',

	/** ELO expected score scale factor */
	eloScaleFactor: 400,

	/** Maximum players shown on leaderboard */
	topPlayersLimit: 10
} as const;
