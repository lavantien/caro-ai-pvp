<script lang="ts">
	import { onMount } from 'svelte';
	import Board from '$lib/components/Board.svelte';
	import PlayerTimerStrip from '$lib/components/PlayerTimerStrip.svelte';
	import MoveNotation from '$lib/components/MoveNotation.svelte';
	import GameSettings from '$lib/components/GameSettings.svelte';
	import GameResultBanner from '$lib/components/GameResultBanner.svelte';
	import { GameStore } from '$lib/stores/gameStore.svelte';
	import { soundManager } from '$lib/utils/sound';
	import { ApiConfig } from '$lib/config/apiConfig';
	import { GameConfig } from '$lib/config/gameConfig';
	import { switchPlayer } from '$lib/types/game';
	import type { Player, Cell } from '$lib/types/game';
	import type { GameMode, TimeControl, UCIConnectionStatus, DifficultyLevel } from '$lib/types/game';

	let store = new GameStore();
	let gameId = $state<string>('');
	let loading = $state(true);
	let error = $state<string>('');
	let errorMessage = $state<string>('');
	let winningLine = $state<Array<{ x: number; y: number }>>([]);
	let lastMove = $state<{ x: number; y: number } | null>(null);

	let redTime = $state(180);
	let blueTime = $state(180);

	let gameMode = $state<GameMode>('pvp');
	let timeControl = $state<TimeControl>('7+5');
	let aiSide = $state<'red' | 'blue'>('blue');
	let difficulty = $state<DifficultyLevel>(5);
	let isAiThinking = $state(false);
	let moveInProgress = $state(false);

	let useUCIForAI = $state(false);
	let uciConnectionStatus = $state<UCIConnectionStatus>('disconnected');

	function showError(msg: string) {
		errorMessage = msg;
		setTimeout(() => errorMessage = '', 5000);
	}

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

	function toggleUCI() {
		if (uciConnectionStatus === 'connected') {
			disconnectUCI();
		} else {
			connectUCI();
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
					gameMode: gameMode,
					...(gameMode === 'pvai'
						? { [aiSide === 'red' ? 'redDifficulty' : 'blueDifficulty']: difficulty }
						: gameMode === 'aivai'
						? { difficulty: difficulty }
						: {})
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

			// Trigger first AI move for AIvAI mode
			if (gameMode === 'aivai' && !store.isGameOver) {
				makeAiMove();
			}
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
			}
			// Chain next AI move for AIvAI mode
			if (gameMode === 'aivai' && !store.isGameOver) {
				makeAiMove();
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
	<div class="flex items-center justify-center min-h-screen">
		<p class="text-lg text-gray-500">Loading game...</p>
	</div>
{:else if error}
	<div class="flex items-center justify-center min-h-screen px-4">
		<div class="text-center">
			<p class="text-lg text-red-500">Error: {error}</p>
			<p class="mt-2 text-sm text-gray-500">Make sure the backend API is running on {ApiConfig.baseUrl}</p>
		</div>
	</div>
{:else}
	<div class="flex flex-col items-center min-h-screen px-1 sm:px-4 py-2 gap-2">
		{#if errorMessage}
			<div class="w-full max-w-[1024px] p-2 bg-red-100 border border-red-300 rounded-lg flex justify-between items-center">
				<p class="text-red-700 text-sm">{errorMessage}</p>
				<button onclick={() => errorMessage = ''} class="text-red-500 hover:text-red-700 ml-2 text-lg">&times;</button>
			</div>
		{/if}

		<GameSettings
			bind:gameMode
			bind:timeControl
			bind:aiSide
			bind:difficulty
			moveNumber={store.moveNumber}
			{uciConnectionStatus}
			{useUCIForAI}
			{isAiThinking}
			onToggleUCI={toggleUCI}
			onToggleUseUCI={() => useUCIForAI = !useUCIForAI}
			onNewGame={createNewGame}
		/>

		<!-- Current turn indicator -->
		<div class="text-sm text-gray-600">
			<span class="font-medium uppercase {store.currentPlayer === 'red' ? 'text-red-600' : 'text-blue-600'}">{store.currentPlayer}</span>
			&middot; Move {store.moveNumber}
			{#if store.moveNumber > 0}
				<button
					onclick={handleUndo}
					disabled={!gameId || store.moveNumber === 0 || store.isGameOver}
					class="ml-2 px-2 py-0.5 text-xs bg-gray-200 text-gray-700 rounded disabled:opacity-40"
				>
					Undo
				</button>
			{/if}
		</div>

		<!-- Opponent timer (top) -->
		<PlayerTimerStrip
			player="blue"
			timeRemaining={blueTime}
			isActive={store.currentPlayer === 'blue' && !store.isGameOver}
			onTimeOut={() => handleTimeOut('blue')} />

		<!-- Board -->
		<Board board={store.board} onMove={handleMove} {winningLine} {lastMove} />

		<!-- Player timer (bottom) -->
		<PlayerTimerStrip
			player="red"
			timeRemaining={redTime}
			isActive={store.currentPlayer === 'red' && !store.isGameOver}
			onTimeOut={() => handleTimeOut('red')} />

		<!-- Move notation -->
		<MoveNotation moves={store.moveHistory} currentMoveNumber={store.moveNumber} />
	</div>

	{#if store.isGameOver}
		<GameResultBanner winner={store.winner} onNewGame={createNewGame} />
	{/if}
{/if}
