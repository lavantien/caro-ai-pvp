import { HapticConfig } from '$lib/config/hapticConfig';

export function vibrate(duration: number | number[]): void {
	if ('vibrate' in navigator) {
		navigator.vibrate(duration);
	}
}

export function vibrateOnValidMove(): void {
	vibrate(HapticConfig.validMoveDuration);
}

export function vibrateOnInvalidMove(): void {
	vibrate([...HapticConfig.invalidMovePattern]);
}
