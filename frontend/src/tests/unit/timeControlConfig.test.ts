import { describe, expect, it } from 'vitest';
import { TIME_CONTROLS, DEFAULT_TIME_CONTROL, timeControlOption, timeControlLabel, type TimeControl } from '$lib/config';

describe('timeControlConfig', () => {
	it('defines the six controls with value, label, clock, and increment', () => {
		expect(TIME_CONTROLS).toEqual([
			{ value: '1+0', label: '1+0 Bullet', initialTimeMs: 60_000, incrementSeconds: 0 },
			{ value: '3+0', label: '3+0 Blitz', initialTimeMs: 180_000, incrementSeconds: 0 },
			{ value: '3+2', label: '3+2 Blitz', initialTimeMs: 180_000, incrementSeconds: 2 },
			{ value: '7+5', label: '7+5 Rapid', initialTimeMs: 420_000, incrementSeconds: 5 },
			{ value: '10+0', label: '10+0 Rapid', initialTimeMs: 600_000, incrementSeconds: 0 },
			{ value: '15+10', label: '15+10 Classical', initialTimeMs: 900_000, incrementSeconds: 10 }
		]);
	});

	it('has unique values', () => {
		const values = TIME_CONTROLS.map((tc) => tc.value);
		expect(new Set(values).size).toBe(values.length);
	});

	it('defaults to 7+5 and the default is a member of the list', () => {
		expect(DEFAULT_TIME_CONTROL).toBe('7+5');
		expect(timeControlOption(DEFAULT_TIME_CONTROL)?.initialTimeMs).toBe(420_000);
	});

	it('resolves options by value and rejects unknown values', () => {
		expect(timeControlOption('3+2')?.incrementSeconds).toBe(2);
		expect(timeControlOption('2+1')).toBeUndefined();
		expect(timeControlOption('bullet')).toBeUndefined();
	});

	it('labels known values and passes unknown values through', () => {
		expect(timeControlLabel('15+10')).toBe('15+10 Classical');
		expect(timeControlLabel('??')).toBe('??');
	});

	it('accepts every list value as the TimeControl union', () => {
		const accepted: TimeControl[] = TIME_CONTROLS.map((tc) => tc.value);
		expect(accepted).toHaveLength(6);
	});
});
