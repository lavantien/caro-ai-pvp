import { browser } from '$app/environment';
import { writable } from 'svelte/store';
import { GameConfig } from '$lib/config/gameConfig';

interface PlayerRating {
	name: string;
	rating: number;
	gamesPlayed: number;
	wins: number;
	losses: number;
	lastPlayed: string;
}

interface RatingData {
	currentPlayer: PlayerRating | null;
	leaderboard: PlayerRating[];
}

const DEFAULT_RATING = GameConfig.defaultEloRating;
const STORAGE_KEY = 'caro-ratings';

function createRatingStore() {
	const { subscribe, update, set } = writable<RatingData>({
		currentPlayer: null,
		leaderboard: []
	});

	// Load from localStorage on client side
	if (browser) {
		loadFromStorage();
	}

	function loadFromStorage() {
		try {
			const stored = localStorage.getItem(STORAGE_KEY);
			if (stored) {
				const data = JSON.parse(stored);
				set(data);
			}
		} catch (e) {
			console.error('Failed to load ratings:', e);
		}
	}

	function saveToStorage(data: RatingData) {
		try {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
		} catch (e) {
			console.error('Failed to save ratings:', e);
		}
	}

	function createPlayer(name: string) {
		update((data) => {
			const newPlayer: PlayerRating = {
				name,
				rating: DEFAULT_RATING,
				gamesPlayed: 0,
				wins: 0,
				losses: 0,
				lastPlayed: new Date().toISOString()
			};

			const newData: RatingData = {
				...data,
				currentPlayer: newPlayer,
				leaderboard: [...data.leaderboard, newPlayer]
			};

			saveToStorage(newData);
			return newData;
		});
	}

	function updateRating(won: boolean, opponentRating: number = DEFAULT_RATING) {
		update((data) => {
			if (!data.currentPlayer) return data;

			// Simple ELO calculation
			const K = GameConfig.eloKFactor;
			const expectedScore =
				1 / (1 + Math.pow(10, (opponentRating - data.currentPlayer.rating) / 400));
			const actualScore = won ? 1 : 0;
			const ratingChange = Math.round(K * (actualScore - expectedScore));

			const updatedPlayer: PlayerRating = {
				...data.currentPlayer,
				rating: data.currentPlayer.rating + ratingChange,
				gamesPlayed: data.currentPlayer.gamesPlayed + 1,
				wins: won ? data.currentPlayer.wins + 1 : data.currentPlayer.wins,
				losses: won ? data.currentPlayer.losses : data.currentPlayer.losses + 1,
				lastPlayed: new Date().toISOString()
			};

			// Update leaderboard
			const leaderboard = data.leaderboard
				.filter((p) => p.name !== updatedPlayer.name)
				.concat(updatedPlayer)
				.sort((a, b) => b.rating - a.rating)
				.slice(0, 10); // Keep top 10

			const newData: RatingData = {
				currentPlayer: updatedPlayer,
				leaderboard
			};

			saveToStorage(newData);
			return newData;
		});
	}

	function getTopPlayers(count: number = 10): PlayerRating[] {
		let topPlayers: PlayerRating[] = [];
		const unsubscribe = subscribe((data) => {
			topPlayers = data.leaderboard.slice(0, count);
		});
		unsubscribe();
		return topPlayers;
	}

	return {
		subscribe,
		createPlayer,
		updateRating,
		getTopPlayers,
		set
	};
}

export const ratingStore = createRatingStore();
