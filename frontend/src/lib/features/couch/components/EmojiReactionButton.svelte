<script lang="ts">
	import { Popover } from 'bits-ui';
	import { SmilePlus } from 'lucide-svelte';
	import { couch } from '$lib/features/couch/couch.svelte';
	import EmojiPicker from './EmojiPicker.svelte';
	import * as m from '$lib/paraglide/messages';

	let { portalTo, triggerClass = '' }: { portalTo?: HTMLElement; triggerClass?: string } = $props();

	let open = $state(false);

	function pick(emoji: string) {
		couch.sendEmoji(emoji);
		open = false;
	}
</script>

<div class="group relative flex flex-col items-center">
	<!-- recently-used emojis slide up on hover for quick re-sending (same width) -->
	{#if couch.recentEmojis.length > 0}
		<div
			class="pointer-events-none absolute bottom-full mb-2 flex w-full translate-y-2 flex-col gap-1.5
				opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:translate-y-0
				group-hover:opacity-100"
		>
			{#each couch.recentEmojis as e (e)}
				<button
					class="flex aspect-square w-full items-center justify-center rounded-full border border-edge
						bg-surface-2/90 text-xl shadow-lg backdrop-blur transition-transform hover:scale-110"
					onclick={() => couch.sendEmoji(e)}
					aria-label={m.couch_react()}
				>
					{e}
				</button>
			{/each}
		</div>
	{/if}

	<Popover.Root bind:open>
		<Popover.Trigger class={triggerClass} aria-label={m.couch_react()} title={m.couch_react()}>
			<SmilePlus class="size-5" />
		</Popover.Trigger>
		<Popover.Portal to={portalTo}>
			<Popover.Content
				side="top"
				sideOffset={10}
				class="z-50 animate-pop-in overflow-hidden rounded-card border border-edge shadow-xl"
			>
				{#if open}
					<EmojiPicker onpick={pick} />
				{/if}
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
</div>
