<script lang="ts">
	import { onMount } from 'svelte';
	import Board from '$lib/components/Board.svelte';
	import Timer from '$lib/components/Timer.svelte';
	import SoundToggle from '$lib/components/SoundToggle.svelte';
	import MoveHistory from '$lib/components/MoveHistory.svelte';
	import Leaderboard from '$lib/components/Leaderboard.svelte';
	import { GameStore } from '$lib/stores/gameStore.svelte';
	import { ratingStore } from '$lib/stores/ratingStore.svelte';
	import { soundManager } from '$lib/utils/sound';
	import type { GameState } from '$lib/types/game';

	let store = new GameStore();
	let gameId = $state<string>('');
	let loading = $state(true);
	let error = $state<string>('');
	let winningLine = $state<Array<{ x: number; y: number }>>([]);

	// Timer values from backend
	let redTime = $state(180);
	let blueTime = $state(180);

	// Player registration
	const DEFAULT_RATING = 1500;
	let playerName = $state('');
	let showNameInput = $state(false);
	let currentPlayer = $state<{ name: string; rating: number } | null>(null);

	// Game mode: PvP, PvAI, or AIvAI
	type GameMode = 'pvp' | 'pvai' | 'aivai';
	type TimeControl = '1+0' | '3+2' | '7+5' | '15+10';
	type AIDifficulty = 'Braindead' | 'Easy' | 'Medium' | 'Hard' | 'Grandmaster' | 'Experimental';

	let gameMode = $state<GameMode>('pvp');
	let timeControl = $state<TimeControl>('7+5');
	let aiDifficulty = $state<AIDifficulty>('Medium');
	let aiSide = $state<'red' | 'blue'>('blue');  // For PvAI, which side is human?
	let isAiThinking = $state(false);

	function handleRegisterPlayer() {
		if (playerName.trim()) {
			ratingStore.createPlayer(playerName.trim());
		}
	}

	// Subscribe to rating store
	ratingStore.subscribe((data) => {
		if (data.currentPlayer) {
			currentPlayer = {
				name: data.currentPlayer.name,
				rating: data.currentPlayer.rating
			};
			showNameInput = false;
		}
	});

	onMount(async () => {
		await createNewGame();
	});

	async function createNewGame() {
		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		try {
			// Determine AI difficulties based on game mode
			let redAIDifficulty: string | undefined;
			let blueAIDifficulty: string | undefined;

			switch (gameMode) {
				case 'pvai':
					// Player vs AI - human plays red by default (AI goes first)
					if (aiSide === 'blue') {
						// Human plays blue, AI plays red
						redAIDifficulty = aiDifficulty;
					} else {
						// Human plays red, AI plays blue (default)
						blueAIDifficulty = aiDifficulty;
					}
					break;
				case 'aivai':
					// AI vs AI - both sides are AI
					redAIDifficulty = aiDifficulty;
					blueAIDifficulty = aiDifficulty;
					break;
			}

			// Create new game on backend
			const response = await fetch(`${apiUrl}/api/game/new`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					timeControl: timeControl,
					gameMode: gameMode,
					redAIDifficulty: redAIDifficulty,
					blueAIDifficulty: blueAIDifficulty
				})
			});

			if (!response.ok) throw new Error('Failed to create game');

			const data = await response.json();
			gameId = data.gameId;

			// Initialize timer values from backend response
			if (data.state.initialTime) {
				redTime = data.state.initialTime;
				blueTime = data.state.initialTime;
			}

			// Sync with backend state
			await syncWithBackend();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function syncWithBackend() {
		if (!gameId) return;

		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';
		const response = await fetch(`${apiUrl}/api/game/${gameId}`);
		const data = await response.json();

		// Update local state with backend state
		store.board = data.state.board;
		store.currentPlayer = data.state.currentPlayer;
		store.moveNumber = data.state.moveNumber;
		store.isGameOver = data.state.isGameOver;
		redTime = data.state.redTimeRemaining;
		blueTime = data.state.blueTimeRemaining;
		if (data.state.winner) {
			store.winner = data.state.winner;
		}
		if (data.state.winningLine) {
			winningLine = data.state.winningLine;
		}
	}

	async function handleMove(x: number, y: number) {
		if (store.isGameOver || !gameId) return;

		// Optimistic board update only (not move history or state)
		const cell = store.board.find((c) => c.x === x && c.y === y);
		if (!cell || cell.player !== 'none') return;

		const previousPlayer = store.currentPlayer;
		cell.player = previousPlayer;

		// Play stone placement sound (previousPlayer is always "red" or "blue" at this point)
		soundManager.playStoneSound(previousPlayer as "red" | "blue");

		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		try {
			const response = await fetch(`${apiUrl}/api/game/${gameId}/move`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ x, y })
			});

			if (!response.ok) {
				const errorText = await response.text();
				// Revert board cell on error
				cell.player = 'none';
				alert(errorText);
				return;
			}

			const data = await response.json();

			// Update local state with server response (authoritative)
			store.board = data.state.board;
			store.currentPlayer = data.state.currentPlayer;
			store.moveNumber = data.state.moveNumber;
			store.isGameOver = data.state.isGameOver;
			redTime = data.state.redTimeRemaining;
			blueTime = data.state.blueTimeRemaining;

			// Add to move history after successful move
			store.moveHistory = [
				...store.moveHistory,
				{
					moveNumber: data.state.moveNumber,
					player: previousPlayer,
					x,
					y
				}
			];

			if (data.state.winningLine) {
				winningLine = data.state.winningLine;
			}

			if (data.state.isGameOver && data.state.winner) {
				store.winner = data.state.winner;
				soundManager.playWinSound(data.state.winner);
				alert(`${data.state.winner.toUpperCase()} WINS!`);

				// Update player rating (for local PvP, both players get the same opponent rating)
				if (currentPlayer) {
					const playerWon = store.currentPlayer === data.state.winner;
					ratingStore.updateRating(playerWon, DEFAULT_RATING);
				}
			}
		} catch (err) {
			// Revert board cell on error
			cell.player = 'none';
			alert('Failed to make move');
		}

		// If playing against AI and game not over, trigger AI move
		// PvAI mode with human as red (default) - AI plays blue
		// PvAI mode with human as blue - AI plays red
		// AIvAI mode - both sides are AI (not fully implemented on frontend yet)
		const aiPlayer = gameMode === 'pvai' && aiSide === 'red' ? 'red' : 'blue';
		if ((gameMode === 'pvai' || gameMode === 'aivai') && !store.isGameOver && store.currentPlayer === aiPlayer) {
			makeAiMove();
		}
	}

	async function makeAiMove() {
		if (!gameId || store.isGameOver) return;

		isAiThinking = true;
		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		// Store previous board to find AI's move
		const previousBoard = [...store.board];
		const currentPlayer = store.currentPlayer;

		try {
			const response = await fetch(`${apiUrl}/api/game/${gameId}/ai-move`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ difficulty: aiDifficulty })
			});

			if (!response.ok) {
				const errorText = await response.text();
				alert(errorText);
				return;
			}

			const data = await response.json();

			// Find which cell changed (AI's move)
			let aiMove = { x: 0, y: 0 };
			for (let i = 0; i < store.board.length; i++) {
				if (previousBoard[i].player === 'none' && data.state.board[i].player !== 'none') {
					aiMove = { x: data.state.board[i].x, y: data.state.board[i].y };
					break;
				}
			}

			// Update local state with server response
			store.board = data.state.board;
			store.currentPlayer = data.state.currentPlayer;
			store.moveNumber = data.state.moveNumber;
			store.isGameOver = data.state.isGameOver;
			redTime = data.state.redTimeRemaining;
			blueTime = data.state.blueTimeRemaining;

			// Add AI move to history (use the actual player from the move)
			store.moveHistory = [
				...store.moveHistory,
				{
					moveNumber: data.state.moveNumber,
					player: currentPlayer as 'red' | 'blue',
					x: aiMove.x,
					y: aiMove.y
				}
			];

			if (data.state.winningLine) {
				winningLine = data.state.winningLine;
			}

			if (data.state.isGameOver && data.state.winner) {
				store.winner = data.state.winner;
				soundManager.playWinSound(data.state.winner);
				alert(`${data.state.winner.toUpperCase()} WINS!`);

				// Update player rating (AI always has DEFAULT_RATING)
				if (currentPlayer) {
					const playerWon = data.state.winner === 'red';
					ratingStore.updateRating(playerWon, DEFAULT_RATING);
				}
			}
		} catch (err) {
			alert('Failed to make AI move');
		} finally {
			isAiThinking = false;
		}
	}

	async function handleUndo() {
		if (!gameId || store.isGameOver) return;

		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		try {
			const response = await fetch(`${apiUrl}/api/game/${gameId}/undo`, {
				method: 'POST'
			});

			if (!response.ok) {
				const errorText = await response.text();
				alert(errorText);
				return;
			}

			const data = await response.json();

			// Update local state with server response
			store.board = data.state.board;
			store.currentPlayer = data.state.currentPlayer;
			store.moveNumber = data.state.moveNumber;
			store.isGameOver = data.state.isGameOver;
			redTime = data.state.redTimeRemaining;
			blueTime = data.state.blueTimeRemaining;

			// Clear winning line if present
			winningLine = [];
		} catch (err) {
			alert('Failed to undo move');
		}
	}

	function handleTimeOut(player: string) {
		if (store.isGameOver) return;

		store.isGameOver = true;
		const winner = player === 'red' ? 'blue' : 'red';
		store.winner = winner;
		alert(`${winner.toUpperCase()} WINS! ${player.toUpperCase()} ran out of time.`);
	}
</script>

{#if loading}
	<div class="container mx-auto p-8 text-center">
		<p class="text-xl">Loading game...</p>
	</div>
{:else if error}
	<div class="container mx-auto p-8 text-center">
		<p class="text-xl text-red-500">Error: {error}</p>
		<p class="mt-4">Make sure the backend API is running on http://localhost:5207</p>
		<p class="text-sm text-gray-500">API URL: {import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207'}</p>
	</div>
{:else}
	<div class="container mx-auto p-4 max-w-4xl">
		<div class="flex justify-between items-center mb-4">
			<h1 class="text-2xl font-bold text-gray-800">Caro Game</h1>
			<div class="flex gap-2">
				<button
					onclick={handleUndo}
					disabled={!gameId || store.moveNumber === 0 || store.isGameOver}
					class="px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
				>
					Undo
				</button>
				<SoundToggle />
			</div>
		</div>

		<!-- Game Mode Selection -->
		<div class="mb-4 bg-gray-50 border border-gray-200 rounded-lg p-4">
			<div class="flex flex-wrap gap-4 items-center justify-between">
				<div class="flex flex-wrap gap-2 items-center">
					<!-- Game Mode Buttons -->
					<div class="flex gap-2">
						<button
							onclick={() => gameMode = 'pvp'}
							class="px-4 py-2 rounded transition-colors {gameMode === 'pvp'
								? 'bg-blue-600 text-white'
								: 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-100'}"
							disabled={store.moveNumber > 0}
						>
							Player vs Player
						</button>
						<button
							onclick={() => gameMode = 'pvai'}
							class="px-4 py-2 rounded transition-colors {gameMode === 'pvai'
								? 'bg-blue-600 text-white'
								: 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-100'}"
							disabled={store.moveNumber > 0}
						>
							Player vs AI
						</button>
						<button
							onclick={() => gameMode = 'aivai'}
							class="px-4 py-2 rounded transition-colors {gameMode === 'aivai'
								? 'bg-blue-600 text-white'
								: 'bg-white text-gray-700 border border-gray-300 hover:bg-gray-100'}"
							disabled={store.moveNumber > 0}
						>
							AI vs AI
						</button>
					</div>

					<!-- Time Control Selector -->
					<div class="flex items-center gap-2 ml-4">
						<label for="time-control" class="text-sm font-medium text-gray-700">Time Control:</label>
						<select
							id="time-control"
							bind:value={timeControl}
							class="px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
							disabled={store.moveNumber > 0}
						>
							<option value="1+0">1+0 (Bullet)</option>
							<option value="3+2">3+2 (Blitz)</option>
							<option value="7+5">7+5 (Rapid)</option>
							<option value="15+10">15+10 (Classical)</option>
						</select>
					</div>

					{#if gameMode === 'pvai' || gameMode === 'aivai'}
						<div class="flex items-center gap-2">
							<label for="ai-difficulty" class="text-sm font-medium text-gray-700">AI Difficulty:</label>
							<select
								id="ai-difficulty"
								bind:value={aiDifficulty}
								class="px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
								disabled={store.moveNumber > 0}
							>
								<option value="Braindead">Braindead</option>
								<option value="Easy">Easy</option>
								<option value="Medium">Medium</option>
								<option value="Hard">Hard</option>
								<option value="Grandmaster">Grandmaster</option>
								<option value="Experimental">Experimental</option>
							</select>
						</div>
					{/if}

					{#if gameMode === 'pvai'}
						<div class="flex items-center gap-2">
							<label for="ai-side" class="text-sm font-medium text-gray-700">You play as:</label>
							<select
								id="ai-side"
								bind:value={aiSide}
								class="px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
								disabled={store.moveNumber > 0}
							>
								<option value="blue">Red (AI goes first)</option>
								<option value="red">Blue (Human goes first)</option>
							</select>
						</div>
					{/if}
				</div>

				{#if isAiThinking}
					<div class="flex items-center gap-2 text-blue-600">
						<svg
							class="animate-spin h-5 w-5"
							xmlns="http://www.w3.org/2000/svg"
							fill="none"
							viewBox="0 0 24 24"
						>
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path
								class="opacity-75"
								fill="currentColor"
								d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
							></path>
						</svg>
						<span class="text-sm font-medium">AI is thinking...</span>
					</div>
				{/if}
			</div>
		</div>

		<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
			<Timer
				player="red"
				timeRemaining={redTime}
				isActive={store.currentPlayer === 'red' && !store.isGameOver}
				onTimeOut={() => handleTimeOut('red')}
				gameId={gameId} />
			<Timer
				player="blue"
				timeRemaining={blueTime}
				isActive={store.currentPlayer === 'blue' && !store.isGameOver}
				onTimeOut={() => handleTimeOut('blue')}
				gameId={gameId} />
		</div>

		<div class="mb-4 text-center">
			<p class="text-lg">
				Current Player: <strong class="uppercase {store.currentPlayer === 'red'
						? 'text-red-600'
						: 'text-blue-600'}">{store.currentPlayer}</strong>
				(Move #{store.moveNumber})
			</p>
		</div>

		<div class="flex justify-center">
			<Board board={store.board} onMove={handleMove} winningLine={winningLine} />
		</div>

		<div class="mt-6">
			<MoveHistory moves={store.moveHistory} currentMoveNumber={store.moveNumber} />
		</div>

		<div class="mt-6">
			{#if currentPlayer}
				<div class="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-4">
					<div class="flex justify-between items-center">
						<div>
							<p class="text-sm text-gray-600">Playing as</p>
							<p class="text-lg font-bold text-blue-900">{currentPlayer.name}</p>
						</div>
						<div class="text-right">
							<p class="text-sm text-gray-600">Rating</p>
							<p class="text-2xl font-bold text-blue-900">{currentPlayer.rating}</p>
						</div>
					</div>
				</div>
			{:else}
				<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
					<p class="text-gray-700 mb-2">Track your rating on the leaderboard!</p>
					<div class="flex gap-2">
						<input
							type="text"
							bind:value={playerName}
							placeholder="Enter your name"
							class="flex-1 px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
							onkeypress={(e) => e.key === 'Enter' && handleRegisterPlayer()}
						/>
						<button
							onclick={handleRegisterPlayer}
							class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition-colors"
						>
							Register
						</button>
					</div>
				</div>
			{/if}

			<Leaderboard limit={5} />
		</div>

		{#if store.isGameOver}
			<div class="mt-4 p-4 bg-green-100 rounded text-center">
				<h2 class="text-2xl font-bold uppercase text-green-800">{store.winner} WINS!</h2>
			</div>
		{/if}
	</div>
{/if}
