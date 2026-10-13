<script lang="ts">
	import { lockupSvg, type LockupLayout } from '@couchverse/broadcast/brand/wordmark';
	import { CHANNEL_NAME } from '../channel';

	// The Couch TV lockup, the Couchverse logo with couchtv, as the broadcast draws it (its corner
	// bug among them): a row for small sizes, a stack (the logo above the word, as every ident
	// ends) for a page of its own. It comes from the package's brand module alone: the package's
	// index brings the templates and the audio engine, which would weigh on every page with a
	// lockup (home's card among them). Its letters are outlines, so it needs no font.
	let {
		size,
		layout = 'row',
		class: cls = ''
	}: {
		/** The word's size, px: a row is 1.06 of it tall, a stack about 3.25. */
		size: number;
		layout?: LockupLayout;
		class?: string;
	} = $props();

	const svg = $derived(lockupSvg({ size, layout, label: CHANNEL_NAME }));
</script>

<span class="lockup inline-block {cls}">
	<!-- our own static markup: the brand's lockup, a pure function of fixed options -->
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html svg}
</span>

<style>
	.lockup :global(svg) {
		display: block;
	}
</style>
