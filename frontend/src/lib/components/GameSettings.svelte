<script lang="ts">
	import type { GameMode, TimeControl, UCIConnectionStatus, DifficultyLevel } from '$lib/types/game';
	import { difficultyName } from '$lib/types/game';
	import { GameConfig } from '$lib/config/gameConfig';
	import { UIConfig } from '$lib/config/uiConfig';
	import { TIME_CONTROLS, timeControlLabel } from '$lib/config/timeControlConfig';

	interface Props {
		gameMode: GameMode;
		timeControl: TimeControl;
		aiSide: 'red' | 'blue';
		difficulty: DifficultyLevel;
		moveNumber: number;
		uciConnectionStatus: UCIConnectionStatus;
		useUCIForAI: boolean;
		isAiThinking: boolean;
		onToggleUCI: () => void;
		onToggleUseUCI: () => void;
		onNewGame: () => void;
	}

	let {
		gameMode = $bindable(),
		timeControl = $bindable(),
		aiSide = $bindable(),
		difficulty = $bindable(),
		moveNumber,
		uciConnectionStatus,
		useUCIForAI,
		isAiThinking,
		onToggleUCI,
		onToggleUseUCI,
		onNewGame
	}: Props = $props();

	let isOpen = $state(true);

	$effect(() => {
		if (moveNumber > 0) {
			isOpen = false;
		}
	});

	function modeLabel(mode: GameMode): string {
		switch (mode) {
			case 'pvp': return 'PvP';
			case 'pvai': return 'PvAI';
			case 'aivai': return 'AI vs AI';
		}
	}
</script>

<div class="w-full mx-auto px-1" style="max-width: {UIConfig.maxContentWidthPx}px;">
	<button
		onclick={() => isOpen = !isOpen}
		class="flex items-center justify-between w-full px-3 py-2 rounded-lg bg-gray-50 border border-gray-200 text-sm"
	>
		<span class="text-gray-600">
			{modeLabel(gameMode)} &middot; {timeControlLabel(timeControl)}
		</span>
		<svg
			class="w-4 h-4 text-gray-500 transition-transform {isOpen ? 'rotate-180' : ''}"
			fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
			<path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
		</svg>
	</button>

	{#if isOpen}
		<div class="mt-2 p-3 bg-gray-50 border border-gray-200 rounded-lg space-y-3">
			<div class="flex flex-wrap gap-2">
				<button
					onclick={() => gameMode = 'pvp'}
					class="px-3 py-1.5 rounded text-sm font-medium transition-colors {gameMode === 'pvp'
						? 'bg-blue-600 text-white'
						: 'bg-white text-gray-700 border border-gray-300'}"
					disabled={moveNumber > 0}
				>
					PvP
				</button>
				<button
					onclick={() => gameMode = 'pvai'}
					class="px-3 py-1.5 rounded text-sm font-medium transition-colors {gameMode === 'pvai'
						? 'bg-blue-600 text-white'
						: 'bg-white text-gray-700 border border-gray-300'}"
					disabled={moveNumber > 0}
				>
					PvAI
				</button>
				<button
					onclick={() => gameMode = 'aivai'}
					class="px-3 py-1.5 rounded text-sm font-medium transition-colors {gameMode === 'aivai'
						? 'bg-blue-600 text-white'
						: 'bg-white text-gray-700 border border-gray-300'}"
					disabled={moveNumber > 0}
				>
					AI vs AI
				</button>
			</div>

			<div class="flex flex-wrap items-center gap-2">
				<select
					bind:value={timeControl}
					class="px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
					disabled={moveNumber > 0}
				>
					{#each TIME_CONTROLS as tc (tc.value)}
						<option value={tc.value}>{tc.label}</option>
					{/each}
				</select>

				{#if gameMode === 'pvai'}
					<select
						bind:value={aiSide}
						class="px-2 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
						disabled={moveNumber > 0}
					>
						<option value="blue">Red first</option>
						<option value="red">Blue first</option>
					</select>
				{/if}
			</div>

			{#if gameMode === 'pvai' || gameMode === 'aivai'}
				<div class="flex items-center gap-2">
					<label for="difficulty" class="text-xs text-gray-500 whitespace-nowrap">
						AI: {difficultyName(difficulty)}
					</label>
					<input
						id="difficulty"
						type="range" min={GameConfig.minDifficulty} max={GameConfig.maxDifficulty} step="1"
						bind:value={difficulty}
						disabled={moveNumber > 0}
						class="flex-1 h-1.5 bg-gray-300 rounded-lg appearance-none cursor-pointer accent-blue-600"
					/>
				</div>
			{/if}

			<div class="flex items-center gap-2 flex-wrap">
				<button
					onclick={onToggleUCI}
					class="px-2.5 py-1.5 rounded text-xs font-medium transition-colors {uciConnectionStatus === 'connected'
						? 'bg-green-600 text-white'
						: uciConnectionStatus === 'connecting'
							? 'bg-yellow-500 text-white'
							: 'bg-gray-400 text-white'}"
					disabled={uciConnectionStatus === 'connecting'}
				>
					UCI: {uciConnectionStatus === 'connected' ? 'On' : uciConnectionStatus === 'connecting' ? '...' : 'Off'}
				</button>
				{#if uciConnectionStatus === 'connected'}
					<button
						onclick={onToggleUseUCI}
						class="px-2.5 py-1.5 rounded text-xs font-medium transition-colors {useUCIForAI
							? 'bg-blue-600 text-white'
							: 'bg-gray-200 text-gray-700'}"
					>
						{useUCIForAI ? 'UCI Active' : 'Use API'}
					</button>
				{/if}

				{#if isAiThinking}
					<span class="flex items-center gap-1 text-xs text-blue-600">
						<svg class="animate-spin h-3.5 w-3.5" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						Thinking...
					</span>
				{/if}

				<button
					onclick={onNewGame}
					class="ml-auto px-3 py-1.5 bg-green-600 text-white rounded text-sm font-medium hover:bg-green-700 transition-colors"
				>
					New Game
				</button>
			</div>
		</div>
	{/if}
</div>
