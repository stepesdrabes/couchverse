<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { prefersReducedMotion } from 'svelte/motion';
	import AchievementCard from './AchievementCard.svelte';
	import { rank } from '../rank.svelte';

	// Renders nothing: it turns the core's celebrations into sonner toasts. It stands down
	// entirely while an in-player overlay is mounted, because the root Toaster is a fixed
	// root-layout element and would vanish in fullscreen.
	//
	// The toast is created once the take settles, out of the effect flush: creating one writes
	// sonner's own reactive state, and doing that while our effect is still running corrupts
	// its height bookkeeping.
	rank.watch();

	let taking = $state(false);

	$effect(() => {
		if (rank.overlays > 0 || !rank.celebration || taking) return;
		taking = true;
		void rank.take().then((next) => {
			taking = false;
			if (!next) return;
			toast.custom(AchievementCard, {
				componentProps: { achievement: next },
				// nothing is moving to cue the read, so leave it up a little longer
				duration: prefersReducedMotion.current ? 6500 : 6000,
				unstyled: true
			});
		});
	});
</script>
