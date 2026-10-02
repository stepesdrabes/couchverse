<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { backOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import AchievementCard from './AchievementCard.svelte';
	import { rank } from '../rank.svelte';
	import type { Achievement } from '../types';

	// The in-player celebration. It must be mounted inside the player wrapper: a
	// fixed root-layout element (like the sonner Toaster) is not part of the
	// fullscreen subtree and would simply not be visible, which is the same trap
	// the couch bar documents.
	let { class: cls = 'absolute top-6 right-6 z-40' }: { class?: string } = $props();

	let showing = $state<Achievement | null>(null);

	onMount(() => {
		rank.overlayMounts++;
		return () => {
			rank.overlayMounts--;
		};
	});

	// Taking from the queue is deferred out of the effect flush for the same
	// reason the toast path defers: an effect should not write the state it reads
	// while it is still running.
	$effect(() => {
		if (showing || rank.queue.length === 0) return;
		const timer = setTimeout(() => {
			const next = rank.shift();
			if (next) showing = next;
		}, 0);
		return () => clearTimeout(timer);
	});

	$effect(() => {
		if (!showing) return;
		const ttl = prefersReducedMotion.current ? 6500 : 4500;
		const timer = setTimeout(() => (showing = null), ttl);
		return () => clearTimeout(timer);
	});
</script>

{#if showing}
	<div
		class={cls}
		in:fly={{ x: 32, duration: 420, easing: backOut }}
		out:fly={{ x: 24, duration: 220 }}
	>
		<AchievementCard achievement={showing} live />
	</div>
{/if}
