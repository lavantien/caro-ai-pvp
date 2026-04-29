<script lang="ts">
	import { UIConfig } from '$lib/config/uiConfig';

	interface Position {
		x: number;
		y: number;
	}

	interface Props {
		winningLine: Position[];
		boardSize: number;
		cellSize: number;
		labelSize?: number;
	}

	let { winningLine, boardSize, cellSize, labelSize = 0 }: Props = $props();

	const svgWidth = $derived(boardSize * cellSize);
	const svgHeight = $derived(boardSize * cellSize);
</script>

{#if winningLine.length >= 2}
	<div class="absolute pointer-events-none" style="left: {labelSize}px; top: {labelSize}px; width: {svgWidth}px; height: {svgHeight}px;">
		<svg width={svgWidth} height={svgHeight} class="w-full h-full">
			<line
				x1={winningLine[0].x * cellSize + cellSize / 2}
				y1={winningLine[0].y * cellSize + cellSize / 2}
				x2={winningLine[winningLine.length - 1].x * cellSize + cellSize / 2}
				y2={winningLine[winningLine.length - 1].y * cellSize + cellSize / 2}
				stroke={UIConfig.winningLineColor}
				stroke-width={UIConfig.winningLineStrokeWidth}
				stroke-linecap="round"
				class="winning-line"
				style="stroke-dasharray: 1000; stroke-dashoffset: 1000; animation: drawLine {UIConfig.winningLineAnimationMs}ms ease-out forwards;"
			/>
		</svg>
	</div>
{/if}

<style>
	@keyframes drawLine {
		to {
			stroke-dashoffset: 0;
		}
	}
</style>
