import { paraglideVitePlugin } from '@inlang/paraglide-js';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			strategy: ['localStorage', 'baseLocale']
		}),
		tailwindcss(),
		sveltekit()
	],
	server: {
		proxy: {
			'/api': 'http://localhost:8080'
		}
	},
	test: {
		include: ['src/**/*.test.ts']
	}
});
