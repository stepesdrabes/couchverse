<script lang="ts">
	import { onMount } from 'svelte';

	let { onpick }: { onpick: (emoji: string) => void } = $props();
	let el = $state<HTMLElement>();

	onMount(() => {
		// register the bundled <emoji-picker> custom element (no CDN)
		import('emoji-picker-element');
		const handler = (e: Event) => {
			const detail = (e as CustomEvent<{ unicode?: string }>).detail;
			if (detail?.unicode) onpick(detail.unicode);
		};
		el?.addEventListener('emoji-click', handler);
		return () => el?.removeEventListener('emoji-click', handler);
	});
</script>

<!-- data-source points at the self-hosted dataset under static/, never a CDN -->
<emoji-picker bind:this={el} class="dark" data-source="/emoji/data.json"></emoji-picker>

<style>
	emoji-picker {
		--background: var(--color-surface-2);
		--border-color: transparent;
		--input-border-color: var(--color-edge);
		--indicator-color: var(--color-accent);
		--button-active-background: var(--color-surface);
		--button-hover-background: var(--color-surface);
		height: 22rem;
		width: 20rem;
		border: none;
	}
</style>
