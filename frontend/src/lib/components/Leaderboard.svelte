<script lang="ts">
	import { ratingStore } from '$lib/stores/ratingStore.svelte';

	interface Player {
		name: string;
		rating: number;
		gamesPlayed: number;
		wins: number;
		losses: number;
	}

	let { limit = 10 }: { limit?: number } = $props();

	let leaderboard = $state<Player[]>([]);
	let currentPlayer = $state<Player | null>(null);

	// Subscribe to store updates
	ratingStore.subscribe((data) => {
		leaderboard = data.leaderboard.slice(0, limit);
		currentPlayer = data.currentPlayer;
	});

	function getWinRate(player: Player): string {
		if (player.gamesPlayed === 0) return '0%';
		return ((player.wins / player.gamesPlayed) * 100).toFixed(1) + '%';
	}
</script>

<div class="bg-white rounded-lg shadow-lg p-6">
	<h2 class="text-2xl font-bold text-gray-800 mb-4">Leaderboard (Top {limit})</h2>

	{#if leaderboard.length === 0}
		<p class="text-gray-500 text-center py-4">No players yet. Be the first to play!</p>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full">
				<thead>
					<tr class="border-b-2 border-gray-200">
						<th class="text-left py-2 px-4 font-semibold text-gray-700">Rank</th>
						<th class="text-left py-2 px-4 font-semibold text-gray-700">Player</th>
						<th class="text-right py-2 px-4 font-semibold text-gray-700">Rating</th>
						<th class="text-right py-2 px-4 font-semibold text-gray-700">W-L</th>
						<th class="text-right py-2 px-4 font-semibold text-gray-700">Win Rate</th>
					</tr>
				</thead>
				<tbody>
					{#each leaderboard as player, index}
						<tr
							class="border-b border-gray-100 {currentPlayer?.name === player.name
								? 'bg-blue-50'
								: 'hover:bg-gray-50'} transition-colors"
						>
							<td class="py-2 px-4">
								{#if index === 0}
									<span class="text-2xl" aria-label="Gold medal">🥇</span>
								{:else if index === 1}
									<span class="text-2xl" aria-label="Silver medal">🥈</span>
								{:else if index === 2}
									<span class="text-2xl" aria-label="Bronze medal">🥉</span>
								{:else}
									<span class="text-gray-600 font-semibold">{index + 1}</span>
								{/if}
							</td>
							<td class="py-2 px-4 font-medium {currentPlayer?.name === player.name
								? 'text-blue-700'
								: 'text-gray-900'}">
								{player.name}
								{#if currentPlayer?.name === player.name}
									<span class="ml-2 text-xs bg-blue-600 text-white px-2 py-1 rounded">You</span>
								{/if}
							</td>
							<td class="py-2 px-4 text-right font-mono font-bold {currentPlayer?.name === player.name
								? 'text-blue-700'
								: 'text-gray-900'}">
								{player.rating}
							</td>
							<td class="py-2 px-4 text-right text-gray-600">{player.wins}-{player.losses}</td>
							<td class="py-2 px-4 text-right text-gray-600">{getWinRate(player)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
