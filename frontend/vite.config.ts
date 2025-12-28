import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		strictPort: true, // Fail if port is in use instead of trying next port
		host: true
	},
	test: {
		include: ['src/**/*.{test,spec}.{js,ts}'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json', 'html']
		}
	}
});
