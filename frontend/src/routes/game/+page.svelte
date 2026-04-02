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
	import { ApiConfig } from '$lib/config/apiConfig';
	import { GameConfig } from '$lib/config/gameConfig';
	import { switchPlayer } from '$lib/types/game';
	import type { Player, Cell } from '$lib/types/game';
	import type { GameMode, TimeControl, UCIConnectionStatus } from '$lib/types/game';

	let store = new GameStore();
	let gameId = $state<string>('');
	let loading = $state(true);
	let error = $state<string>('');
	let errorMessage = $state<string>('');
	let winningLine = $state<Array<{ x: number; y: number }>>([]);
	let lastMove = $state<{ x: number; y: number } | null>(null);

	let redTime = $state(180);
	let blueTime = $state(180);

	const DEFAULT_RATING = GameConfig.defaultEloRating;
	let playerName = $state('');
	let showNameInput = $state(false);
	let currentPlayer = $state<{ name: string; rating: number } | null>(null);

	let gameMode = $state<GameMode>('pvp');
	let timeControl = $state<TimeControl>('7+5');
	let aiSide = $state<'red' | 'blue'>('blue');
	let isAiThinking = $state(false);
	let moveInProgress = $state(false);

	let useUCIForAI = $state(false);
	let uciConnectionStatus = $state<UCIConnectionStatus>('disconnected');

	function showError(msg: string) {
		errorMessage = msg;
		setTimeout(() => errorMessage = '', 5000);
	}

	function handleRegisterPlayer() {
		if (playerName.trim()) {
			ratingStore.createPlayer(playerName.trim());
		}
	}

	ratingStore.subscribe((data) => {
		if (data.currentPlayer) {
			currentPlayer = {
				name: data.currentPlayer.name,
				rating: data.currentPlayer.rating
			};
			showNameInput = false;
		}
	});

	async function connectUCI() {
		uciConnectionStatus = 'connecting';
		try {
			const connected = await store.connectUCI();
			uciConnectionStatus = connected ? 'connected' : 'disconnected';
			if (connected) {
				store.reset();
			}
		} catch (err) {
			console.error('Failed to connect to UCI:', err);
			uciConnectionStatus = 'disconnected';
		}
	}

	function disconnectUCI() {
		store.disconnectUCI();
		uciConnectionStatus = 'disconnected';
	}

	async function toggleUCI() {
		if (uciConnectionStatus === 'connected') {
			disconnectUCI();
		} else {
			await connectUCI();
		}
	}

	function syncGameState(state: Record<string, any>) {
		store.board = state.board;
		store.currentPlayer = state.currentPlayer;
		store.moveNumber = state.moveNumber;
		store.isGameOver = state.isGameOver;
		if (state.redTimeRemaining > 0) {
			redTime = state.redTimeRemaining;
		}
		if (state.blueTimeRemaining > 0) {
			blueTime = state.blueTimeRemaining;
		}
		if (state.winningLine) {
			winningLine = state.winningLine;
		}
	}

	function handleGameEnd(winner: 'red' | 'blue') {
		store.winner = winner;
		soundManager.playWinSound(winner);
	}

	function findNewMove(oldBoard: Cell[], newBoard: Cell[]): { x: number; y: number } {
		for (let i = 0; i < oldBoard.length; i++) {
			if (oldBoard[i].player === 'none' && newBoard[i].player !== 'none') {
				return { x: newBoard[i].x, y: newBoard[i].y };
			}
		}
		return { x: 0, y: 0 };
	}

	onMount(async () => {
		await createNewGame();
	});

	async function createNewGame() {
		try {
			store.reset();
			winningLine = [];
			lastMove = null;
			isAiThinking = false;

			const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.newGame}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					timeControl: timeControl,
					gameMode: gameMode
				})
			});

			if (!response.ok) throw new Error('Failed to create game');

			const data = await response.json();
			gameId = data.gameId;

			if (data.state.initialTime) {
				redTime = data.state.initialTime;
				blueTime = data.state.initialTime;
			}

			await syncWithBackend();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function syncWithBackend() {
		if (!gameId) return;

		const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.game(gameId)}`);
		const data = await response.json();
		syncGameState(data.state);
		if (data.state.winner) {
			store.winner = data.state.winner;
		}
	}

	async function handleMove(x: number, y: number) {
		if (store.isGameOver || !gameId || moveInProgress) return;

		const cell = store.board[x * GameConfig.boardSize + y];
		if (!cell || cell.player !== 'none') return;

		moveInProgress = true;

		const previousPlayer = store.currentPlayer;
		cell.player = previousPlayer;

		soundManager.playStoneSound(previousPlayer as 'red' | 'blue');

		try {
			const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.move(gameId)}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ x, y })
			});

			if (!response.ok) {
				const errorText = await response.text();
				cell.player = 'none';
				showError(errorText);
				return;
			}

			const data = await response.json();

			syncGameState(data.state);

			store.moveHistory.push({
				moveNumber: data.state.moveNumber,
				player: previousPlayer,
				x,
				y
			});

			lastMove = { x, y };
			if (data.state.isGameOver && data.state.winner) {
				handleGameEnd(data.state.winner);
				if (currentPlayer) {
					const playerWon = previousPlayer === data.state.winner;
					ratingStore.updateRating(playerWon, DEFAULT_RATING);
				}
			}
		} catch (err) {
			cell.player = 'none';
			showError('Failed to make move');
		} finally {
			moveInProgress = false;
		}

		const aiPlayer = gameMode === 'pvai' && aiSide === 'red' ? 'red' : 'blue';
		if ((gameMode === 'pvai' || gameMode === 'aivai') && !store.isGameOver && store.currentPlayer === aiPlayer) {
			makeAiMove();
		}
	}

	async function makeAiMove() {
		if (!gameId || store.isGameOver) return;

		isAiThinking = true;

		const previousBoard = store.board;
		const aiPlayer = store.currentPlayer;

		try {
			let aiMove = { x: 0, y: 0 };
			let data;

			if (useUCIForAI && uciConnectionStatus === 'connected') {
				try {
					const move = await store.getAIMoveUCI();
					if (move) {
						aiMove = move;
						const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.move(gameId)}`, {
							method: 'POST',
							headers: { 'Content-Type': 'application/json' },
							body: JSON.stringify({ x: move.x, y: move.y })
						});
						if (!response.ok) {
							throw new Error('Failed to apply UCI move');
						}
						data = await response.json();
					} else {
						useUCIForAI = false;
						throw new Error('UCI move failed');
					}
				} catch (uciError) {
					console.warn('UCI move failed, falling back to API:', uciError);
					const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.aiMove(gameId)}`, {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({})
					});
					if (!response.ok) {
						showError(await response.text());
						return;
					}
					data = await response.json();
					aiMove = findNewMove(previousBoard, data.state.board);
				}
			} else {
				const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.aiMove(gameId)}`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({})
				});

				if (!response.ok) {
					showError(await response.text());
					return;
				}

				data = await response.json();
				aiMove = findNewMove(previousBoard, data.state.board);
			}

			syncGameState(data.state);

			store.moveHistory.push({
				moveNumber: data.state.moveNumber,
				player: aiPlayer as 'red' | 'blue',
				x: aiMove.x,
				y: aiMove.y
			});

			lastMove = { x: aiMove.x, y: aiMove.y };
			if (data.state.isGameOver && data.state.winner) {
				handleGameEnd(data.state.winner);
				if (currentPlayer && gameMode === 'pvai') {
					const humanSide = aiSide === 'red' ? 'blue' : 'red';
					const playerWon = data.state.winner === humanSide;
					ratingStore.updateRating(playerWon, DEFAULT_RATING);
				}
			}
		} catch (err) {
			showError('Failed to make AI move');
		} finally {
			isAiThinking = false;
		}
	}

	async function handleUndo() {
		if (!gameId || store.isGameOver) return;

		try {
			const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.undo(gameId)}`, {
				method: 'POST'
			});

			if (!response.ok) {
				showError(await response.text());
				return;
			}

			const data = await response.json();
			syncGameState(data.state);
			winningLine = [];
			lastMove = null;
		} catch (err) {
			showError('Failed to undo move');
		}
	}

	function handleTimeOut(player: string) {
		if (store.isGameOver) return;

		store.isGameOver = true;
		const winner = switchPlayer(player as Player);
		store.winner = winner;
	}
</script>

{#if loading}
	<div class="container mx-auto p-8 text-center">
		<p class="text-xl">Loading game...</p>
	</div>
{:else if error}
	<div class="container mx-auto p-8 text-center">
		<p class="text-xl text-red-500">Error: {error}</p>
		<p class="mt-4">Make sure the backend API is running on {ApiConfig.baseUrl}</p>
		<p class="text-sm text-gray-500">API URL: {ApiConfig.baseUrl}</p>
	</div>
{:else}
	<div class="container mx-auto p-4 max-w-4xl">
		{#if errorMessage}
			<div class="mb-4 p-3 bg-red-100 border border-red-300 rounded-lg flex justify-between items-center">
				<p class="text-red-700 text-sm">{errorMessage}</p>
				<button onclick={() => errorMessage = ''} class="text-red-500 hover:text-red-700 ml-2">&times;</button>
			</div>
		{/if}

		<div class="flex justify-between items-center mb-4">
			<h1 class="text-2xl font-bold text-gray-800">Caro Game</h1>
			<div class="flex gap-2 items-center">
				<!-- UCI Connection Status -->
				<div class="flex items-center gap-2 mr-4">
					<span class="text-sm text-gray-600">UCI Engine:</span>
					<button
						onclick={toggleUCI}
						class="px-3 py-1 rounded text-sm font-medium transition-colors {
							uciConnectionStatus === 'connected'
								? 'bg-green-600 text-white hover:bg-green-700'
								: uciConnectionStatus === 'connecting'
									? 'bg-yellow-500 text-white'
									: 'bg-gray-400 text-white hover:bg-gray-500'
						}"
						disabled={uciConnectionStatus === 'connecting'}
					>
						{uciConnectionStatus === 'connected' ? 'Connected' :
						 uciConnectionStatus === 'connecting' ? 'Connecting...' : 'Connect'}
					</button>
					{#if uciConnectionStatus === 'connected'}
						<button
							onclick={() => useUCIForAI = !useUCIForAI}
							class="px-3 py-1 rounded text-sm font-medium ml-1 transition-colors {
								useUCIForAI
									? 'bg-blue-600 text-white hover:bg-blue-700'
									: 'bg-gray-300 text-gray-700 hover:bg-gray-400'
							}"
						>
							{useUCIForAI ? 'UCI Active' : 'Use API'}
						</button>
					{/if}
				</div>
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

					{#if gameMode === 'pvai'}
						<div class="flex items-center gap-2">
							<label for="ai-side" class="text-sm font-medium text-gray-700">You play as:</label>
							<select
								id="ai-side"
								bind:value={aiSide}
								class="px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
								disabled={store.moveNumber > 0}
							>
								<option value="blue">Red (you go first)</option>
								<option value="red">Blue (you go second)</option>
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
				onTimeOut={() => handleTimeOut('red')} />
			<Timer
				player="blue"
				timeRemaining={blueTime}
				isActive={store.currentPlayer === 'blue' && !store.isGameOver}
				onTimeOut={() => handleTimeOut('blue')} />
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
			<Board board={store.board} onMove={handleMove} winningLine={winningLine} lastMove={lastMove} />
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
				<button
					onclick={createNewGame}
					class="mt-3 px-6 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
				>
					New Game
				</button>
			</div>
		{/if}
	</div>
{/if}
