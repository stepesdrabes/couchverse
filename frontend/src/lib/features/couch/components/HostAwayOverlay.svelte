<script lang="ts">
	import { fade } from 'svelte/transition';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { couch } from '$lib/features/couch/couch.svelte';
	import BouncingDvd from './BouncingDvd.svelte';
	import * as m from '$lib/paraglide/messages';

	const host = $derived(couch.host);
</script>

<div
	transition:fade={{ duration: 200 }}
	class="absolute inset-0 z-30 flex flex-col items-center justify-center gap-5 overflow-hidden bg-black/90 px-6 text-center"
>
	<!-- easter egg: the classic bouncing DVD screensaver while we wait -->
	<BouncingDvd />

	<div class="relative z-10 flex flex-col items-center gap-5">
		{#if host}
			<UserAvatar
				name={host.displayName}
				avatarId={host.avatarId ?? null}
				seed={host.seed}
				class="size-20 rounded-2xl text-2xl"
			/>
			<p class="text-lg font-semibold text-white">
				{m.couch_host_choosing({ name: host.displayName })}
			</p>
		{:else}
			<p class="text-lg font-semibold text-white">{m.couch_host_choosing_generic()}</p>
		{/if}
	</div>
</div>
