import { describe, it, expect } from 'vitest';
import { switchPlayer, difficultyName } from '$lib/types/game';
import type { DifficultyLevel } from '$lib/types/game';

describe('switchPlayer', () => {
	it('switches red to blue', () => {
		expect(switchPlayer('red')).toBe('blue');
	});

	it('switches blue to red', () => {
		expect(switchPlayer('blue')).toBe('red');
	});

	it('switches none to red', () => {
		expect(switchPlayer('none')).toBe('red');
	});
});

describe('difficultyName', () => {
	const cases: [DifficultyLevel, string][] = [
		[1, 'Novice'],
		[2, 'Beginner'],
		[3, 'Intermediate'],
		[4, 'Advanced'],
		[5, 'Grandmaster'],
	];

	for (const [level, name] of cases) {
		it(`returns "${name}" for level ${level}`, () => {
			expect(difficultyName(level)).toBe(name);
		});
	}
});
