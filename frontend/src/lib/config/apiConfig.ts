/**
 * Centralized API configuration - single source of truth for backend endpoints.
 */

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';
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
