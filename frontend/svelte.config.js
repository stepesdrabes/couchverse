import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	// SCSS in component <style lang="scss"> blocks
	preprocess: vitePreprocess(),
	kit: {
		// Static SPA build: the Go binary embeds `build/` and serves index.html
		// as the fallback for every non-/api route.
		adapter: adapter({ fallback: 'index.html' })
	}
};

export default config;
