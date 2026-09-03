import { sveltekit } from '@sveltejs/kit/vite';
import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';

export default defineConfig(({ mode }) => {
	// Empty prefix so non-VITE_ vars like FRONTEND_PORT are visible.
	const env = loadEnv(mode, '.', '');

	return {
		plugins: [sveltekit()],
		server: {
			// 5173 mirrors scripts/lib.mjs DEFAULT_FRONTEND_PORT; importing
			// lib.mjs here would drag node builtins into svelte-check.
			port: Number(env.FRONTEND_PORT ?? 5173),
			strictPort: true, // Fail if port is in use instead of trying next port
			host: true
		},
		test: {
			include: ['src/**/*.{test,spec}.{js,ts}'],
			coverage: {
				provider: 'v8',
				reporter: ['text', 'json', 'json-summary', 'html']
			}
		}
	};
});
