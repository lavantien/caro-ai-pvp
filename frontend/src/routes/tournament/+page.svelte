<script lang="ts">
	import { tournamentStore } from '$lib/stores/tournamentStore.svelte';
	import { onMount, onDestroy } from 'svelte';
	import CellComponent from '$lib/components/Cell.svelte';
	import type { ThinkingStats } from '$lib/stores/tournamentStore.svelte';

	onMount(async () => {
		try {
			await tournamentStore.connect();
		} catch (err) {
			console.error('Failed to connect to tournament hub:', err);
		}
	});

	onDestroy(() => {
		tournamentStore.disconnect();
	});

	function getDifficultyColor(diff: string): string {
		const colors: Record<string, string> = {
			beginner: 'bg-gray-200 text-gray-800',
			easy: 'bg-green-200 text-green-800',
			normal: 'bg-blue-200 text-blue-800',
			medium: 'bg-yellow-200 text-yellow-800',
			hard: 'bg-orange-200 text-orange-800',
			harder: 'bg-red-200 text-red-800',
			veryHard: 'bg-purple-200 text-purple-800',
			expert: 'bg-pink-200 text-pink-800',
			master: 'bg-indigo-200 text-indigo-800',
			grandmaster: 'bg-cyan-200 text-cyan-800',
			legend: 'bg-amber-200 text-amber-800'
		};
		return colors[diff] || 'bg-gray-200 text-gray-800';
	}

	function getDifficultyLabel(diff: string): string {
		const labels: Record<string, string> = {
			beginner: 'D1',
			easy: 'D2',
			normal: 'D3',
			medium: 'D4',
			hard: 'D5',
			harder: 'D6',
			veryHard: 'D7',
			expert: 'D8',
			master: 'D9',
			grandmaster: 'D10',
			legend: 'D11'
		};
		return labels[diff] || diff;
	}

	// Determine which player made the last move based on move number (odd = red, even = blue after first move)
	// Red always moves first (move 1), so odd moves are red, even moves are blue
	function lastMovePlayerColor(): 'red' | 'blue' {
		const moveNumber = tournamentStore.state.currentMatch?.moveNumber || 0;
		return moveNumber % 2 === 1 ? 'red' : 'blue';
	}

	// Determine which player is currently thinking (opposite of last move player)
	function currentPlayerColor(): 'red' | 'blue' {
		return lastMovePlayerColor() === 'red' ? 'blue' : 'red';
	}

	// Check if the player who just moved has stats available
	function hasLastMoverStats(): boolean {
		const lastMover = lastMovePlayerColor();
		return lastMover === 'red'
			? tournamentStore.state.currentMatch?.redLastStats !== null
			: tournamentStore.state.currentMatch?.blueLastStats !== null;
	}

	// Get stats for the player who JUST completed their move (not the one thinking)
	function getLastMoverStats() {
		const lastMover = lastMovePlayerColor();
		if (lastMover === 'red') {
			return tournamentStore.state.currentMatch?.redLastStats;
		}
		return tournamentStore.state.currentMatch?.blueLastStats;
	}

	// Get time remaining for the player who is currently thinking
	function getThinkingPlayerTime(): number {
		const thinker = currentPlayerColor();
		return thinker === 'red'
			? (tournamentStore.state.currentMatch?.redTimeRemainingMs || 0)
			: (tournamentStore.state.currentMatch?.blueTimeRemainingMs || 0);
	}

	// Check if red has made any moves (has stats)
	function hasRedStats(): boolean {
		return tournamentStore.state.currentMatch?.redLastStats !== null;
	}

	// Check if blue has made any moves (has stats)
	function hasBlueStats(): boolean {
		return tournamentStore.state.currentMatch?.blueLastStats !== null;
	}

	// Check if red is currently pondering
	function isRedPondering(): boolean {
		return tournamentStore.state.currentMatch?.redPondering ?? false;
	}

	// Check if blue is currently pondering
	function isBluePondering(): boolean {
		return tournamentStore.state.currentMatch?.bluePondering ?? false;
	}

	// Get current thinking stats (real-time)
	function getCurrentThinkingStats(): ThinkingStats | null {
		return tournamentStore.state.currentMatch?.currentThinkingStats ?? null;
	}

	// Format large numbers with K/M suffixes
	function formatNumber(num: number): string {
		if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
		if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
		return num.toString();
	}
</script>

<div class="max-h-[2060px] bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900 p-4 md:p-6 overflow-hidden">
	<div class="max-w-[1800px] mx-auto">
		<!-- Header -->
		<header class="mb-4">
			<h1 class="text-3xl font-bold text-white mb-1">AI Tournament</h1>
			<p class="text-slate-400 text-sm">22 bots battle in a round-robin tournament (462 games total)</p>
		</header>

		<!-- Connection & Status Bar -->
		<div class="bg-slate-800 rounded-lg p-4 mb-4 flex flex-wrap items-center justify-between gap-3">
			<div class="flex items-center gap-4">
				<div class="flex items-center gap-2">
					<span class="w-3 h-3 rounded-full" class:bg-green-500={tournamentStore.state.connectionState === 'connected'}
						class:bg-yellow-500={tournamentStore.state.connectionState === 'connecting' || tournamentStore.state.connectionState === 'reconnecting'}
						class:bg-red-500={tournamentStore.state.connectionState === 'disconnected'}></span>
					<span class="text-slate-300 text-base">
						{tournamentStore.state.connectionState === 'connected'
							? 'Connected'
							: tournamentStore.state.connectionState === 'connecting'
								? 'Connecting...'
								: tournamentStore.state.connectionState === 'reconnecting'
									? 'Reconnecting...'
									: 'Disconnected'}
					</span>
				</div>

				<div class="px-4 py-1.5 rounded-full text-base font-medium" class:bg-green-600={tournamentStore.state.status === 'running'}
					class:bg-yellow-600={tournamentStore.state.status === 'paused'}
					class:bg-blue-600={tournamentStore.state.status === 'idle'}
					class:bg-purple-600={tournamentStore.state.status === 'completed'}>
					{tournamentStore.state.status.toUpperCase()}
				</div>
			</div>

			<div class="flex items-center gap-2">
				{#if tournamentStore.state.status === 'idle'}
					<button
						disabled={tournamentStore.state.connectionState !== 'connected'}
						onclick={() => tournamentStore.startTournament()}
						class:opacity-50={tournamentStore.state.connectionState !== 'connected'}
						class:cursor-not-allowed={tournamentStore.state.connectionState !== 'connected'}
						class="px-4 py-2 bg-green-600 hover:bg-green-700 disabled:hover:bg-green-600 text-white rounded-lg font-medium transition-colors text-sm">
						Start Tournament
					</button>
				{:else if tournamentStore.state.status === 'running'}
					<button
						onclick={() => tournamentStore.pauseTournament()}
						class="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded-lg font-medium transition-colors text-sm">
						Pause
					</button>
				{:else if tournamentStore.state.status === 'paused'}
					<button
						onclick={() => tournamentStore.resumeTournament()}
						class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-medium transition-colors text-sm">
						Resume
					</button>
				{/if}
			</div>
		</div>

		<!-- Error Message -->
		{#if tournamentStore.state.errorMessage}
			<div class="bg-red-900/50 border border-red-500 text-red-200 px-4 py-2 rounded-lg mb-4 text-sm">
				{tournamentStore.state.errorMessage}
			</div>
		{/if}

		<!-- Main Grid -->
		<div class="grid grid-cols-1 xl:grid-cols-4 gap-4">
			<!-- Left Column: Current Match & Board (full width on large screens) -->
			<div class="xl:col-span-3 space-y-4">
				<!-- Progress Bar -->
				<div class="bg-slate-800 rounded-lg p-3">
					<div class="flex justify-between text-sm text-slate-400 mb-1">
						<span>Progress</span>
						<span>{tournamentStore.state.progress.completed} / {tournamentStore.state.progress.total} games</span>
					</div>
					<div class="w-full bg-slate-700 rounded-full h-2 overflow-hidden">
						<div
							class="h-full bg-gradient-to-r from-blue-500 to-purple-500 transition-all duration-500"
							style="width: {tournamentStore.state.progress.percent}%"></div>
					</div>
					<div class="text-right text-sm text-slate-400 mt-1">{tournamentStore.state.progress.percent.toFixed(1)}%</div>
				</div>

				<!-- Current Match -->
				<div class="bg-slate-800 rounded-lg p-3">
					<h2 class="text-lg font-semibold text-white mb-2">Current Match</h2>
					{#if tournamentStore.state.currentMatch}
						<div class="flex items-center justify-between mb-4">
							<div class="text-center flex-1">
								<div class="text-red-400 font-semibold text-base">{tournamentStore.state.currentMatch.redBotName}</div>
								<div class="text-slate-400 text-sm">{getDifficultyLabel(tournamentStore.state.currentMatch.redDifficulty)}</div>
							</div>
							<div class="text-xl font-bold text-slate-500">VS</div>
							<div class="text-center flex-1">
								<div class="text-blue-400 font-semibold text-base">{tournamentStore.state.currentMatch.blueBotName}</div>
								<div class="text-slate-400 text-sm">{getDifficultyLabel(tournamentStore.state.currentMatch.blueDifficulty)}</div>
							</div>
						</div>

						<!-- Times -->
						<div class="flex justify-center gap-8 mb-4">
							<div class="text-center">
								<div class="text-red-400 font-mono text-lg">
									{tournamentStore.formatTime(tournamentStore.state.currentMatch.redTimeRemainingMs)}
								</div>
								<div class="text-slate-500 text-xs">Red Time</div>
							</div>
							<div class="text-center">
								<div class="text-blue-400 font-mono text-lg">
									{tournamentStore.formatTime(tournamentStore.state.currentMatch.blueTimeRemainingMs)}
								</div>
								<div class="text-slate-500 text-xs">Blue Time</div>
							</div>
						</div>

						<!-- Move Counter -->
						<div class="text-center text-slate-400 text-sm mb-4">
							Move: {tournamentStore.state.currentMatch.moveNumber}
						</div>

						<!-- Mini Board (read-only) - 960px for 64x64 cells -->
						<div class="overflow-x-auto" style="min-height: 972px;">
							<div class="flex justify-center min-w-full">
								<div
									class="grid gap-0 bg-amber-100 p-1 rounded shadow-inner"
									style="display: grid; grid-template-columns: repeat(15, 64px); grid-template-rows: repeat(15, 64px); width: 960px; height: 960px;">
								{#each Array.from({ length: 225 }, (_, i) => ({ x: i % 15, y: Math.floor(i / 15) })) as cell}
									{@const boardCell = tournamentStore.state.currentMatch.board.find(b => b.x === cell.x && b.y === cell.y)}
									{@const lastMovePlayerColor = tournamentStore.state.currentMatch.moveNumber % 2 === 1 ? 'red' : 'blue'}
									{@const isLastMove = tournamentStore.state.currentMatch.lastMove?.x === cell.x
										&& tournamentStore.state.currentMatch.lastMove?.y === cell.y
										&& boardCell?.player === lastMovePlayerColor}
									<div
										class="relative bg-amber-100 border {isLastMove ? 'ring-2 ring-yellow-400 bg-yellow-100' : 'border-amber-400'} flex items-center justify-center">
										{#if boardCell?.player === 'red'}
											<div class="w-[58px] h-[58px] rounded-full bg-red-600 shadow-md"></div>
										{:else if boardCell?.player === 'blue'}
											<div class="w-[58px] h-[58px] rounded-full bg-blue-600 shadow-md"></div>
										{/if}
									</div>
								{/each}
							</div>
						</div>
						</div>

						<!-- Engine Stats -->
						{#if tournamentStore.state.currentMatch && (hasRedStats() || hasBlueStats())}
							{@const lastMover = lastMovePlayerColor()}
							{@const lastMoverStats = getLastMoverStats()}
							{@const lastMoverName = lastMover === 'red' ? tournamentStore.state.currentMatch.redBotName : tournamentStore.state.currentMatch.blueBotName}
							{@const thinker = currentPlayerColor()}
							{@const thinkerName = thinker === 'red' ? tournamentStore.state.currentMatch.redBotName : tournamentStore.state.currentMatch.blueBotName}
							{@const waiter = lastMover}
							{@const waiterName = waiter === 'red' ? tournamentStore.state.currentMatch.redBotName : tournamentStore.state.currentMatch.blueBotName}
							{@const currentStats = getCurrentThinkingStats()}
							{@const waiterPondering = waiter === 'red' ? isRedPondering() : isBluePondering()}
							<div class="bg-slate-800 rounded-lg p-4 mt-4">
								<h3 class="text-lg font-semibold text-white mb-3">Engine Stats</h3>

								<!-- Two-column layout for both players -->
								<div class="grid grid-cols-2 gap-4">
									<!-- Last Mover Stats (completed move) -->
									<div class="bg-slate-700/50 rounded p-3">
										<div class="text-sm {lastMover === 'red' ? 'text-red-400' : 'text-blue-400'} font-semibold mb-2 flex items-center gap-2">
											<span class="w-2.5 h-2.5 rounded-full {lastMover === 'red' ? 'bg-red-500' : 'bg-blue-500'}"></span>
											<span>{lastMover === 'red' ? 'Red' : 'Blue'} ({lastMoverName || lastMover})</span>
											<span class="text-xs px-1.5 py-0.5 rounded bg-slate-600 text-slate-300 ml-auto">Moved</span>
										</div>
										{#if hasLastMoverStats()}
											<div class="grid grid-cols-4 gap-1 text-xs">
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">Depth</div>
													<div class="text-white font-mono text-sm">{lastMoverStats?.depthAchieved ?? '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">Nodes</div>
													<div class="text-white font-mono text-sm">{lastMoverStats ? formatNumber(lastMoverStats.nodesSearched) : '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">NPS</div>
													<div class="text-white font-mono text-sm">{lastMoverStats ? formatNumber(Math.round(lastMoverStats.nodesPerSecond)) : '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">TT</div>
													<div class="text-white font-mono text-sm">{lastMoverStats ? (lastMoverStats.tableHitRate).toFixed(0) + '%' : '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">VCF</div>
													<div class="text-white font-mono text-sm">{lastMoverStats?.vcfDepthAchieved && lastMoverStats.vcfDepthAchieved > 0 ? lastMoverStats.vcfDepthAchieved : '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">VCF Nodes</div>
													<div class="text-white font-mono text-sm">{lastMoverStats?.vcfNodesSearched && lastMoverStats.vcfNodesSearched > 0 ? formatNumber(lastMoverStats.vcfNodesSearched) : '-'}</div>
												</div>
											</div>
										{:else}
											<div class="text-slate-500 text-xs italic">No stats yet</div>
										{/if}
									</div>

									<!-- Current Thinking Player Stats -->
									<div class="bg-slate-700/50 rounded p-3">
										<div class="text-sm {thinker === 'red' ? 'text-red-400' : 'text-blue-400'} font-semibold mb-2 flex items-center gap-2">
											<span class="w-2.5 h-2.5 rounded-full {thinker === 'red' ? 'bg-red-500' : 'bg-blue-500'}"></span>
											<span>{thinker === 'red' ? 'Red' : 'Blue'} ({thinkerName || thinker})</span>
											<span class="text-xs px-1.5 py-0.5 rounded bg-yellow-500/20 text-yellow-400 animate-pulse ml-auto">Thinking</span>
										</div>
										{#if currentStats && currentStats.player === thinker}
											<!-- Real-time stats during search -->
											<div class="grid grid-cols-4 gap-1 text-xs">
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">Depth</div>
													<div class="text-white font-mono text-sm">{currentStats.currentDepth}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">Nodes</div>
													<div class="text-white font-mono text-sm">{formatNumber(currentStats.nodesSearched)}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">NPS</div>
													<div class="text-white font-mono text-sm">{formatNumber(Math.round(currentStats.nodesPerSecond))}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">TT</div>
													<div class="text-white font-mono text-sm">{currentStats.tableHitRate.toFixed(0)}%</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">VCF</div>
													<div class="text-white font-mono text-sm">{currentStats.vcfDepthAchieved > 0 ? currentStats.vcfDepthAchieved : '-'}</div>
												</div>
												<div class="text-center">
													<div class="text-slate-500 text-[10px]">Elapsed</div>
													<div class="text-white font-mono text-sm">{(currentStats.elapsedMs / 1000).toFixed(1)}s</div>
												</div>
											</div>
										{:else}
											<div class="text-slate-500 text-xs italic flex items-center gap-2">
												<span class="animate-pulse">●</span>
												Searching...
											</div>
										{/if}
									</div>
								</div>

								<!-- Pondering Status (non-thinking player) -->
								<div class="mt-3 pt-3 border-t border-slate-700">
									{#if waiterPondering}
										<div class="text-sm {waiter === 'red' ? 'text-red-400' : 'text-blue-400'} flex items-center gap-2">
											<span class="w-2 h-2 rounded-full {waiter === 'red' ? 'bg-red-500' : 'bg-blue-500'} animate-pulse"></span>
											<span>{waiter === 'red' ? 'Red' : 'Blue'} pondered while opponent thought</span>
											<span class="text-xs px-1.5 py-0.5 rounded bg-green-500/20 text-green-400 ml-auto">PONDERED</span>
										</div>
									{:else}
										<div class="text-sm text-slate-500 flex items-center gap-2">
											<span class="w-2 h-2 rounded-full bg-slate-600"></span>
											<span>{waiter === 'red' ? 'Red' : 'Blue'} did not ponder</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}
					{:else}
						<div class="text-center text-slate-500 py-10 text-lg">
							{#if tournamentStore.state.status === 'idle'}
								<p>Tournament not started</p>
							{:else if tournamentStore.state.status === 'completed'}
								<p>Tournament completed!</p>
							{:else}
								<p>Waiting for next match...</p>
							{/if}
						</div>
					{/if}
				</div>
			</div>

			<!-- Right Column: Standings & History -->
			<div class="space-y-4">
				<!-- ELO Standings - Compact single-row layout for all 22 bots -->
				<div class="bg-slate-800 rounded-lg p-3 h-[785px] overflow-hidden flex flex-col flex-shrink-0">
					<h2 class="text-lg font-semibold text-white mb-2 px-1">ELO Standings (22 Bots)</h2>
					<div class="overflow-y-auto flex-1 space-y-px">
						{#each tournamentStore.sortedBots as bot, index}
							<div
								class="flex items-center justify-between px-2 py-1 rounded {index < 3
									? 'bg-gradient-to-r from-amber-900/30 to-transparent'
									: 'bg-slate-700/30 hover:bg-slate-700/50'} transition-colors">
								<div class="flex items-center gap-2 flex-1 min-w-0">
									<span class="w-7 text-center text-base shrink-0 {index < 3 ? 'text-amber-400 font-bold' : 'text-slate-500'}">
										{index + 1}
									</span>
									<span class="text-white text-base font-medium truncate">{bot.name}</span>
									<span class="text-base px-1.5 py-0 rounded {getDifficultyColor(bot.difficulty)} shrink-0">
										{getDifficultyLabel(bot.difficulty)}
									</span>
								</div>
								<div class="flex items-center gap-2 shrink-0 ml-1">
									<span class="text-base text-slate-400">{bot.wins}-{bot.losses}-{bot.draws}</span>
									<span class="text-base font-mono {tournamentStore.getELOChangeClass(bot)}">
										{bot.elo}
									</span>
								</div>
							</div>
						{/each}
					</div>
				</div>

				<!-- Recent Matches - 2x2 grid showing 4 matches -->
				<div class="bg-slate-800 rounded-lg p-3">
					<h2 class="text-lg font-semibold text-white mb-3">Recent Matches</h2>
					{#if tournamentStore.recentMatches.length === 0}
						<p class="text-slate-500 text-sm text-center py-4">No matches yet</p>
					{:else}
						<div class="grid grid-cols-2 gap-2">
							{#each tournamentStore.recentMatches.slice(0, 10) as match}
								<div class="bg-slate-700/50 rounded p-2.5 text-sm">
									<div class="flex items-center justify-between mb-1">
										<span class="{match.winner === 'red' ? 'text-red-400' : 'text-blue-400'} font-semibold text-sm">
											{match.winnerBotName || (match.winner === 'red' ? 'Red' : match.winner === 'blue' ? 'Blue' : 'Draw')}
										</span>
										<span class="text-slate-400 text-xs">{match.totalMoves}m</span>
									</div>
									<div class="flex items-center justify-between text-xs text-slate-500">
										<span class="truncate">vs {match.loserBotName || 'Unknown'}</span>
										{#if match.endedByTimeout}
											<span class="text-yellow-400 text-[10px]">TO</span>
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				<!-- Stats Summary -->
				<div class="bg-slate-800 rounded-lg p-5">
					<h2 class="text-xl font-semibold text-white mb-4">Statistics</h2>
					<div class="grid grid-cols-2 gap-4 text-center">
						<div class="bg-slate-700/50 rounded p-4">
							<div class="text-3xl font-bold text-white">{tournamentStore.state.progress.completed}</div>
							<div class="text-slate-400 text-sm">Games Played</div>
						</div>
						<div class="bg-slate-700/50 rounded p-4">
							<div class="text-3xl font-bold text-white">{tournamentStore.sortedBots.length}</div>
							<div class="text-slate-400 text-sm">Bots</div>
						</div>
						<div class="bg-slate-700/50 rounded p-4">
							<div class="text-3xl font-bold text-green-400">{tournamentStore.sortedBots[0]?.elo ?? 600}</div>
							<div class="text-slate-400 text-sm">Top ELO</div>
						</div>
						<div class="bg-slate-700/50 rounded p-4">
							<div class="text-3xl font-bold text-amber-400">
								{tournamentStore.sortedBots.reduce((max, b) => Math.max(max, b.wins), 0)}
							</div>
							<div class="text-slate-400 text-sm">Most Wins</div>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Game Logs - Full Width Bottom Section -->
		<div class="bg-slate-800 rounded-lg p-5 mt-4">
			<div class="flex items-center justify-between mb-3">
				<h2 class="text-lg font-semibold text-white">Game Logs</h2>
				<button
					onclick={() => tournamentStore.state.gameLogs = []}
					class="text-xs text-slate-400 hover:text-white transition-colors"
				>Clear</button>
			</div>
			<div class="bg-slate-900 rounded-lg p-3 max-h-40 overflow-y-auto font-mono text-xs">
				{#if tournamentStore.state.gameLogs.length === 0}
					<p class="text-slate-600 text-center">No logs yet</p>
				{:else}
					{#each tournamentStore.state.gameLogs as log}
						<div class="flex gap-2 {log.level === 'error' ? 'text-red-400' : log.level === 'warning' ? 'text-yellow-400' : 'text-slate-300'}">
							<span class="text-slate-600 shrink-0">[{log.timestamp}]</span>
							<span class="w-10 shrink-0 {log.source === 'red' ? 'text-red-500' : log.source === 'blue' ? 'text-blue-500' : 'text-slate-500'}">{log.source}</span>
							<span class="break-all">{log.message}</span>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</div>
