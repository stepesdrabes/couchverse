<script lang="ts">
	import { features } from '$lib/features/settings/features.svelte';
	import type { PlaybackKind } from '$lib/features/playback/api';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchPopover from './CouchPopover.svelte';
	import CouchIcon from './icons/CouchIcon.svelte';

	let {
		kind,
		id,
		portalTo,
		triggerClass = 'player-btn'
	}: { kind?: PlaybackKind; id?: string; portalTo?: HTMLElement; triggerClass?: string } = $props();
</script>

{#if features.couchEnabled}
	<div class="relative inline-flex">
		<CouchPopover
			{kind}
			{id}
			{portalTo}
			triggerClass="{triggerClass} {couch.active ? 'text-accent!' : ''}"
		>
			{#snippet trigger()}
				<CouchIcon class="size-5" />
			{/snippet}
		</CouchPopover>
		{#if couch.active && couch.count > 0}
			<span
				class="pointer-events-none absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center
					rounded-full bg-accent px-1 text-[10px] font-bold text-[var(--color-on-accent)]"
			>
				{couch.count}
			</span>
		{/if}
	</div>
{/if}
