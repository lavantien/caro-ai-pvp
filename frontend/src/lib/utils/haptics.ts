export function vibrate(duration: number | number[]): void {
	if ('vibrate' in navigator) {
		navigator.vibrate(duration);
	}
}

export function vibrateOnValidMove(): void {
	vibrate(10); // Short, subtle vibration
}

export function vibrateOnInvalidMove(): void {
	vibrate([30, 50, 30]); // Error pattern
}
