import { resolve } from 'node:path';
import { defineConfig } from 'vitest/config';

// The package has two consumers. The web app imports its source (an ES module, compiled by the
// SPA's own Vite), and the render sidecar loads `dist/render.html`: one self-contained page
// (script, styles, fonts and the logo as hashed assets) that exposes `window.couchtvRender`.
export default defineConfig({
	base: './',
	build: {
		target: 'es2022',
		assetsInlineLimit: 0,
		rollupOptions: {
			input: { render: resolve(import.meta.dirname, 'render.html') }
		}
	},
	server: { fs: { allow: ['.'] } },
	test: {
		include: ['test/unit/**/*.test.ts'],
		environment: 'node'
	}
});
