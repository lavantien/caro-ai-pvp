#!/usr/bin/env python3
import re

# Read the file
with open('src/routes/tournament/+page.svelte', 'r') as f:
    content = f.read()

# 1. Update recent matches to show bot names
old_match_display = """{#each tournamentStore.recentMatches.slice(0, 20) as match}
								<div class="bg-slate-700/50 rounded p-2 text-sm">
									<div class="flex items-center justify-between">
										<span class="{match.winner === 'red' ? 'text-red-400' : 'text-blue-400'} font-medium">
											{match.winner === 'red' ? 'Red' : match.winner === 'blue' ? 'Blue' : 'Draw'}
										</span>
										<span class="text-slate-400">{match.totalMoves} moves</span>
									</div>
									{#if match.endedByTimeout}
										<span class="text-xs text-yellow-400">Timeout</span>
									{/if}
								</div>
							{/each}"""

new_match_display = """{#each tournamentStore.recentMatches.slice(0, 20) as match}
								<div class="bg-slate-700/50 rounded p-2 text-sm">
									<div class="flex items-center justify-between mb-1">
										<span class="{match.winner === 'red' ? 'text-red-400' : 'text-blue-400'} font-medium">
											{match.winnerBotName || (match.winner === 'red' ? 'Red' : match.winner === 'blue' ? 'Blue' : 'Draw')}
										</span>
										<span class="text-slate-400">{match.totalMoves} moves</span>
									</div>
									<div class="flex items-center justify-between text-xs text-slate-500">
										<span>vs {match.loserBotName || 'Unknown'}</span>
										{#if match.endedByTimeout}
											<span class="text-yellow-400">Timeout</span>
										{/if}
									</div>
								</div>
							{/each}"""

content = content.replace(old_match_display, new_match_display)

# 2. Update engine stats to 2 rows (grid-cols-4 -> grid-cols-6, 2 rows of 3)
old_stats = """<!-- Engine Stats -->
						{#if tournamentStore.state.currentMatch?.lastMoveStats}
							<div class="bg-slate-800 rounded-lg p-3 mt-4">
								<h3 class="text-sm font-semibold text-white mb-2">Engine Stats (Last Move)</h3>
								<div class="grid grid-cols-4 gap-3 text-xs">
									<div class="text-center">
										<div class="text-slate-400">Depth</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.depthAchieved}</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">NPS</div>
										<div class="text-white font-mono">{Math.round(tournamentStore.state.currentMatch.lastMoveStats.nodesPerSecond).toLocaleString()}</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">TT Hit</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.tableHitRate.toFixed(1)}%</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">Ponder</div>
										<div class="font-mono {tournamentStore.state.currentMatch.lastMoveStats.ponderingActive ? 'text-green-400' : 'text-slate-500'}">
											{tournamentStore.state.currentMatch.lastMoveStats.ponderingActive ? 'ON' : 'OFF'}
										</div>
									</div>
								</div>
							</div>
						{/if}"""

new_stats = """<!-- Engine Stats -->
						{#if tournamentStore.state.currentMatch?.lastMoveStats}
							<div class="bg-slate-800 rounded-lg p-3 mt-4">
								<h3 class="text-sm font-semibold text-white mb-2">Engine Stats (Last Move)</h3>
								<div class="grid grid-cols-6 gap-2 text-xs mb-2">
									<div class="text-center">
										<div class="text-slate-400">Depth</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.depthAchieved}</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">Nodes</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.nodesSearched.toLocaleString()}</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">NPS</div>
										<div class="text-white font-mono">{Math.round(tournamentStore.state.currentMatch.lastMoveStats.nodesPerSecond).toLocaleString()}</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">TT Hit</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.tableHitRate.toFixed(1)}%</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">Ponder</div>
										<div class="font-mono {tournamentStore.state.currentMatch.lastMoveStats.ponderingActive ? 'text-green-400' : 'text-slate-500'}">
											{tournamentStore.state.currentMatch.lastMoveStats.ponderingActive ? 'ON' : 'OFF'}
										</div>
									</div>
									<div class="text-center">
										<div class="text-slate-400">VCF Depth</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.vcfDepthAchieved}</div>
									</div>
								</div>
								<div class="grid grid-cols-3 gap-2 text-xs">
									<div class="text-center col-span-3">
										<div class="text-slate-400">VCF Nodes Searched</div>
										<div class="text-white font-mono">{tournamentStore.state.currentMatch.lastMoveStats.vcfNodesSearched.toLocaleString()}</div>
									</div>
								</div>
							</div>
						{/if}"""

content = content.replace(old_stats, new_stats)

# Write back
with open('src/routes/tournament/+page.svelte', 'w') as f:
    f.write(content)

print("Done")
