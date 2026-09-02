/**
 * Centralized time-control configuration - single source of the select
 * values, display labels, and clock durations. Mirrors backend
 * Caro.Api/TimeControls (minus the legacy Go aliases).
 */

export interface TimeControlOption {
	value: string;
	label: string;
	initialTimeMs: number;
	incrementSeconds: number;
}

export const TIME_CONTROLS = [
	{ value: '1+0', label: '1+0 Bullet', initialTimeMs: 60_000, incrementSeconds: 0 },
	{ value: '3+0', label: '3+0 Blitz', initialTimeMs: 180_000, incrementSeconds: 0 },
	{ value: '3+2', label: '3+2 Blitz', initialTimeMs: 180_000, incrementSeconds: 2 },
	{ value: '7+5', label: '7+5 Rapid', initialTimeMs: 420_000, incrementSeconds: 5 },
	{ value: '10+0', label: '10+0 Rapid', initialTimeMs: 600_000, incrementSeconds: 0 },
	{ value: '15+10', label: '15+10 Classical', initialTimeMs: 900_000, incrementSeconds: 10 }
] as const satisfies readonly TimeControlOption[];

export type TimeControl = (typeof TIME_CONTROLS)[number]['value'];

/** Time control preselected for a new game */
export const DEFAULT_TIME_CONTROL: TimeControl = '7+5';

export function timeControlOption(value: string): Readonly<TimeControlOption> | undefined {
	return TIME_CONTROLS.find((tc) => tc.value === value);
}

export function timeControlLabel(value: string): string {
	return timeControlOption(value)?.label ?? value;
}
