import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// SCSS in component <style lang="scss"> blocks
	preprocess: vitePreprocess(),
	compilerOptions: {
		// our $props-derived initial values are intentional; this warning is noise
		warningFilter: (w) => w.code !== 'state_referenced_locally'
	},
	kit: {
		// Static SPA build: the Go binary embeds `build/` and serves index.html
		// as the fallback for every non-/api route, and the precompressed .br/.gz
		// siblings to clients that accept them (the wasm core shrinks to a third).
		adapter: adapter({ fallback: 'index.html', precompress: true }),
		typescript: {
			// svelte-check type-checks the Playwright suite along with the app
			config: (tsconfig) => {
				tsconfig.include.push('../e2e/**/*.ts', '../playwright.config.ts');
			}
		}
	}
};

export default config;
