<script lang="ts">
	interface Position {
		x: number;
		y: number;
	}

	interface Props {
		winningLine: Position[];
		boardSize: number;
		cellSize: number;
	}

	let { winningLine, boardSize = 15, cellSize = 40 }: Props = $props();

	// Calculate SVG path through winning cells
	function calculatePath(line: Position[]): string {
		if (line.length < 2) return '';

		// Calculate center points of each cell
		const points = line.map((pos) => {
			const centerX = pos.x * cellSize + cellSize / 2;
			const centerY = pos.y * cellSize + cellSize / 2;
			return `${centerX},${centerY}`;
		});

		return `M ${points.join(' L ')}`;
	}

	// SVG dimensions
	const svgWidth = boardSize * cellSize;
	const svgHeight = boardSize * cellSize;
	const pathData = $derived(calculatePath(winningLine));
</script>

{#if winningLine.length >= 2}
	<div class="absolute inset-0 pointer-events-none" style="width: {svgWidth}px; height: {svgHeight}px;">
		<svg width={svgWidth} height={svgHeight} class="w-full h-full">
			<!-- Winning line with animation -->
			<line
				x1={winningLine[0].x * cellSize + cellSize / 2}
				y1={winningLine[0].y * cellSize + cellSize / 2}
				x2={winningLine[winningLine.length - 1].x * cellSize + cellSize / 2}
				y2={winningLine[winningLine.length - 1].y * cellSize + cellSize / 2}
				stroke="#ef4444"
				stroke-width="6"
				stroke-linecap="round"
				class="winning-line"
			/>
		</svg>
	</div>
{/if}

<style>
	.winning-line {
		stroke-dasharray: 1000;
		stroke-dashoffset: 1000;
		animation: drawLine 0.5s ease-out forwards;
	}

	@keyframes drawLine {
		to {
			stroke-dashoffset: 0;
		}
	}
</style>
