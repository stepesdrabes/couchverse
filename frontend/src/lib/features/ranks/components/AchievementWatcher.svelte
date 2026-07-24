<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { prefersReducedMotion } from 'svelte/motion';
	import AchievementCard from './AchievementCard.svelte';
	import { rank } from '../rank.svelte';

	// Renders nothing: it drains the unlock queue into sonner toasts. It stands
	// down entirely while an in-player overlay is mounted, because the root
	// Toaster is a fixed root-layout element and would vanish in fullscreen.
	//
	// The drain is deferred out of the effect flush on purpose: creating a toast
	// writes sonner's own reactive state, and doing that while our effect is
	// still running corrupts its height bookkeeping.
	let draining = false;

	$effect(() => {
		if (rank.overlayMounts > 0 || rank.queue.length === 0 || draining) return;
		draining = true;
		const timer = setTimeout(() => {
			draining = false;
			if (rank.overlayMounts > 0) return;
			const next = rank.shift();
			if (!next) return;
			toast.custom(AchievementCard, {
				componentProps: { achievement: next },
				// nothing is moving to cue the read, so leave it up a little longer
				duration: prefersReducedMotion.current ? 6500 : 6000,
				unstyled: true
			});
		}, 0);
		return () => {
			clearTimeout(timer);
			draining = false;
		};
	});
</script>
