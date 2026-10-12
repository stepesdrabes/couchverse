<script lang="ts">
	import { lockupSvg } from '@couchverse/broadcast/brand/wordmark';
	import { onMount } from 'svelte';
	import { colors } from '$lib/generated/tokens';
	import { CHANNEL_NAME } from '../channel';

	// The C()UCH TV lockup, as the broadcast draws its corner bug. It comes from the package's
	// brand module alone: the package's index brings the templates and the audio engine, which
	// would weigh on every page with a lockup (home's card among them). Its letters are the
	// brand's display face, registered by the package's font loader, fetched once a lockup is
	// on screen; the lockup shows when the face is in.
	let { size, class: cls = '' }: { size: number; class?: string } = $props();

	let ready = $state(false);
	onMount(() => {
		import('@couchverse/broadcast/assets')
			.then(({ ensureFonts }) => ensureFonts())
			.catch(() => {})
			.finally(() => (ready = true));
	});
	const svg = $derived(lockupSvg({ size, brand: colors.accent, label: CHANNEL_NAME }));
</script>

<span class="lockup inline-block {cls}" style="height: {size}px" class:ready>
	<!-- our own static markup: the brand's lockup, a pure function of fixed options -->
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html svg}
</span>

<style>
	.lockup {
		opacity: 0;
		transition: opacity 0.3s ease;
	}

	.lockup.ready {
		opacity: 1;
	}

	.lockup :global(svg) {
		display: block;
	}
</style>
