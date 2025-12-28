<script lang="ts">
	import { onMount } from 'svelte';
	import Board from '$lib/components/Board.svelte';
	import Timer from '$lib/components/Timer.svelte';
	import SoundToggle from '$lib/components/SoundToggle.svelte';
	import MoveHistory from '$lib/components/MoveHistory.svelte';
	import { GameStore } from '$lib/stores/gameStore.svelte';
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

	onMount(async () => {
		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		try {
			// Create new game on backend
			const response = await fetch(`${apiUrl}/api/game/new`, {
				method: 'POST'
			});

			if (!response.ok) throw new Error('Failed to create game');

			const data = await response.json();
			gameId = data.gameId;

			// Sync with backend state
			await syncWithBackend();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Unknown error';
		} finally {
			loading = false;
		}
	});

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

		// Optimistic update
		const success = store.makeMove(x, y);
		if (!success) return;

		// Play stone placement sound (after move validation, currentPlayer is never 'none')
		soundManager.playStoneSound(store.currentPlayer === 'none' ? 'red' : store.currentPlayer);

		const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';

		try {
			const response = await fetch(`${apiUrl}/api/game/${gameId}/move`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ x, y })
			});

			if (!response.ok) {
				const errorText = await response.text();
				alert(errorText);
				// Revert on error
				store.board = store.board.map((c) =>
					c.x === x && c.y === y ? { ...c, player: 'none' as const } : c
				);
				store.moveNumber--;
				store.currentPlayer = store.currentPlayer === 'red' ? 'blue' : 'red';
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

			if (data.state.winningLine) {
				winningLine = data.state.winningLine;
			}

			if (data.state.isGameOver && data.state.winner) {
				store.winner = data.state.winner;
				soundManager.playWinSound(data.state.winner);
				alert(`${data.state.winner.toUpperCase()} WINS!`);
			}
		} catch (err) {
			alert('Failed to make move');
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
			<Board board={store.board} onMove={handleMove} winningLine={winningLine} />
		</div>

		<div class="mt-6">
			<MoveHistory moves={store.moveHistory} currentMoveNumber={store.moveNumber} />
		</div>

		{#if store.isGameOver}
			<div class="mt-4 p-4 bg-green-100 rounded text-center">
				<h2 class="text-2xl font-bold uppercase text-green-800">{store.winner} WINS!</h2>
			</div>
		{/if}
	</div>
{/if}
