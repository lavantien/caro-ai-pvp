<script lang="ts">
	import { onMount } from 'svelte';
	import Board from '$lib/components/Board.svelte';
	import PlayerTimerStrip from '$lib/components/PlayerTimerStrip.svelte';
	import MoveNotation from '$lib/components/MoveNotation.svelte';
	import GameSettings from '$lib/components/GameSettings.svelte';
	import GameResultBanner from '$lib/components/GameResultBanner.svelte';
	import { GameStore } from '$lib/stores/gameStore.svelte';
	import { soundManager } from '$lib/utils/sound';
	import { ApiConfig, GameConfig, UIConfig, E2EConfig, DEFAULT_TIME_CONTROL, timeControlOption } from '$lib/config';
	import type { Cell } from '$lib/types/game';
	import type { GameMode, TimeControl, UCIConnectionStatus, DifficultyLevel } from '$lib/types/game';
	import { difficultyName } from '$lib/types/game';

	let store = new GameStore();
	let gameId = $state<string>('');
	let loading = $state(true);
	let error = $state<string>('');
	let errorMessage = $state<string>('');
	let winningLine = $state<Array<{ x: number; y: number }>>([]);
	let lastMove = $state<{ x: number; y: number } | null>(null);

	const defaultTimeControl = timeControlOption(DEFAULT_TIME_CONTROL)!;

	let redTime = $state(defaultTimeControl.initialTimeMs / 1000);
	let blueTime = $state(defaultTimeControl.initialTimeMs / 1000);

	let gameMode = $state<GameMode>(GameConfig.defaultGameSetup.gameMode);
	let timeControl = $state<TimeControl>(GameConfig.defaultGameSetup.timeControl);
	let aiSide = $state<'red' | 'blue'>(GameConfig.defaultGameSetup.aiSide);
	let difficulty = $state<DifficultyLevel>(GameConfig.defaultGameSetup.difficulty);
	let isAiThinking = $state(false);
	let moveInProgress = $state(false);

	let useUCIForAI = $state(false);
	let uciConnectionStatus = $state<UCIConnectionStatus>('disconnected');

	let redDifficulty = $state<number | null>(null);
	let blueDifficulty = $state<number | null>(null);

	// Bumped whenever a new game is created or the page unmounts, so a
	// stale AI-vs-AI loop stops instead of writing an old game over the
	// fresh one.
	let gameGeneration = 0;

	const openRuleInvalid = $derived(() => {
		if (store.currentPlayer !== 'red' || store.moveNumber !== GameConfig.openRuleSecondMoveNumber) {
			return new Set<string>();
		}
		let redCount = 0;
		let blueCount = 0;
		let firstRedX = 0, firstRedY = 0;
		for (const cell of store.board) {
			if (cell.player === 'red') { redCount++; firstRedX = cell.x; firstRedY = cell.y; }
			else if (cell.player === 'blue') { blueCount++; }
		}
		if (redCount !== 1 || blueCount > 1) return new Set<string>();
		const invalid = new Set<string>();
		for (const cell of store.board) {
			if (cell.player !== 'none') continue;
			const dx = Math.abs(cell.x - firstRedX);
			const dy = Math.abs(cell.y - firstRedY);
			if (dx < GameConfig.openRuleMinDistance && dy < GameConfig.openRuleMinDistance) {
				invalid.add(`${cell.x},${cell.y}`);
			}
		}
		return invalid;
	});

	function aiLabel(side: 'red' | 'blue'): string {
		if (gameMode === 'pvp') return '';
		const diff = side === 'red' ? redDifficulty : blueDifficulty;
		if (diff == null) return '';
		if (gameMode === 'aivai') return `AI (${difficultyName(diff as DifficultyLevel)})`;
		if (gameMode === 'pvai' && aiSide === side) return `AI (${difficultyName(diff as DifficultyLevel)})`;
		return '';
	}

	function showError(msg: string) {
		try {
			const parsed = JSON.parse(msg);
			msg = parsed.message ?? msg;
		} catch {
			// not a backend JSON error body; show as-is
		}
		errorMessage = msg;
		setTimeout(() => errorMessage = '', UIConfig.errorMessageDismissMs);
	}

	async function connectUCI() {
		uciConnectionStatus = 'connecting';
		try {
			const connected = await store.connectUCI();
			uciConnectionStatus = connected ? 'connected' : 'disconnected';
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
		// Zero is a legitimate clock reading: it means the server has flagged
		// a player. Only missing values keep the previous display.
		if (state.redTimeRemaining != null) {
			redTime = state.redTimeRemaining;
		}
		if (state.blueTimeRemaining != null) {
			blueTime = state.blueTimeRemaining;
		}
		if (state.winningLine) {
			winningLine = state.winningLine;
		}
		if (state.redDifficulty != null) {
			redDifficulty = state.redDifficulty;
		}
		if (state.blueDifficulty != null) {
			blueDifficulty = state.blueDifficulty;
		}
	}

	function handleGameEnd(winner: 'red' | 'blue') {
		store.winner = winner;
		soundManager.playWinSound(winner);
	}

	function findNewMove(oldBoard: Cell[], newBoard: Cell[]): { x: number; y: number } | null {
		for (let i = 0; i < oldBoard.length; i++) {
			if (oldBoard[i].player === 'none' && newBoard[i].player !== 'none') {
				return { x: newBoard[i].x, y: newBoard[i].y };
			}
		}
		return null;
	}

	onMount(() => {
		void createNewGame();
		// Leaving the page must stop the AI-vs-AI loop from driving the
		// server any further.
		return () => {
			gameGeneration++;
		};
	});

	function retry() {
		error = '';
		loading = true;
		createNewGame();
	}

	async function createNewGame() {
		// Invalidate any AI loop still running for a previous game.
		gameGeneration++;
		try {
			store.reset();
			winningLine = [];
			lastMove = null;
			isAiThinking = false;
			redDifficulty = null;
			blueDifficulty = null;

			const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.newGame}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					timeControl: timeControl,
					gameMode: gameMode,
					...(gameMode === 'pvai'
						? { [aiSide === 'red' ? 'redDifficulty' : 'blueDifficulty']: difficulty }
						: gameMode === 'aivai'
						? { redDifficulty: difficulty, blueDifficulty: difficulty }
						: {})
				})
			});

			if (!response.ok) throw new Error('Failed to create game');

			const data = await response.json();
			gameId = data.gameId;
			// Debug hook so e2e tests can clean the game up afterwards.
			if (import.meta.env.DEV) {
				(window as any)[E2EConfig.gameIdHookKey] = gameId;
			}

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
		if (data.state.isGameOver) {
			store.winner =
				data.state.winner === 'red' || data.state.winner === 'blue' ? data.state.winner : undefined;
		}
	}

	async function handleMove(x: number, y: number) {
		if (store.isGameOver || !gameId || moveInProgress) return;

		// Spectators cannot inject moves into an engine-vs-engine game, and in
		// player-vs-AI the human may only move on their own turn.
		if (gameMode === 'aivai') return;
		if (gameMode === 'pvai') {
			const aiPlayer = aiSide;
			if (store.currentPlayer === aiPlayer) return;
		}

		const cell = store.board[y * GameConfig.boardSize + x];
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
			if (data.state.isGameOver) {
				if (data.state.winner && data.state.winner !== 'none') {
					handleGameEnd(data.state.winner);
				} else {
					store.winner = undefined;
				}
			}
		} catch (err) {
			cell.player = 'none';
			showError('Failed to make move');
		} finally {
			moveInProgress = false;
		}

		// After the human's move in player-vs-AI, the engine replies.
		if (gameMode === 'pvai' && !store.isGameOver && store.currentPlayer === aiSide) {
			makeAiMove();
		}
	}

	async function makeAiMove() {
		if (!gameId || store.isGameOver) return;

		isAiThinking = true;
		const generation = gameGeneration;
		try {
			// In AI-vs-AI the whole game runs as one loop; in player-vs-AI a
			// single move is made for the AI side. The loop stops when a new
			// game replaces this one or the page unmounts.
			while (!store.isGameOver && generation === gameGeneration) {
				const previousBoard = store.board;
				const aiPlayer = store.currentPlayer;
				let aiMove: { x: number; y: number } | null = null;
				let data;

				if (useUCIForAI && uciConnectionStatus === 'connected') {
					try {
						const move = await store.getAIMoveUCI();
						if (move) {
							const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.move(gameId)}`, {
								method: 'POST',
								headers: { 'Content-Type': 'application/json' },
								body: JSON.stringify({ x: move.x, y: move.y })
							});
							if (!response.ok) {
								throw new Error('Failed to apply UCI move');
							}
							data = await response.json();
							aiMove = move;
						} else {
							useUCIForAI = false;
							throw new Error('UCI move failed');
						}
					} catch (uciError) {
						console.warn('UCI move failed, falling back to API:', uciError);
						showError('UCI engine failed, switching to built-in AI');
					}
				}

				if (!aiMove) {
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

				if (generation !== gameGeneration) break;
				syncGameState(data.state);

				if (aiMove) {
					store.moveHistory.push({
						moveNumber: data.state.moveNumber,
						player: aiPlayer as 'red' | 'blue',
						x: aiMove.x,
						y: aiMove.y
					});
					lastMove = { x: aiMove.x, y: aiMove.y };
					soundManager.playStoneSound(aiPlayer as 'red' | 'blue');
				}

				if (data.state.isGameOver) {
					if (data.state.winner && data.state.winner !== 'none') {
						handleGameEnd(data.state.winner);
					} else {
						store.winner = undefined;
					}
				}
				if (gameMode !== 'aivai') break;
			}
		} catch (err) {
			showError('Failed to make AI move');
		} finally {
			if (generation === gameGeneration) {
				isAiThinking = false;
			}
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
			// Keep notation in sync with the rolled-back board.
			store.moveHistory.pop();
			const last = store.moveHistory[store.moveHistory.length - 1];
			lastMove = last ? { x: last.x, y: last.y } : null;
		} catch (err) {
			showError('Failed to undo move');
		}
	}

	// Local countdowns are display-only: the server owns adjudication, so a
	// flag fall triggers a sync and the authoritative result comes back.
	async function handleTimeOut() {
		if (store.isGameOver) return;
		await syncWithBackend();
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
			<button
				onclick={retry}
				class="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
			>
				Retry
			</button>
		</div>
	</div>
{:else}
	<div class="flex flex-col items-center min-h-screen px-1 sm:px-4 py-2 gap-2">
		{#if errorMessage}
			<div
				class="w-full p-2 bg-red-100 border border-red-300 rounded-lg flex justify-between items-center"
				style="max-width: {UIConfig.maxContentWidthPx}px;"
			>
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
			onTimeOut={handleTimeOut}
			label={aiLabel('blue')} />

		<!-- Board -->
		<Board board={store.board} onMove={handleMove} {winningLine} {lastMove} openRuleInvalid={openRuleInvalid()} />

		<!-- Player timer (bottom) -->
		<PlayerTimerStrip
			player="red"
			timeRemaining={redTime}
			isActive={store.currentPlayer === 'red' && !store.isGameOver}
			onTimeOut={handleTimeOut}
			label={aiLabel('red')} />

		<!-- Move notation -->
		<MoveNotation moves={store.moveHistory} currentMoveNumber={store.moveNumber} />
	</div>

	{#if store.isGameOver}
		<GameResultBanner winner={store.winner} onNewGame={createNewGame} />
	{/if}
{/if}
