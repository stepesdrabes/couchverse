<script lang="ts">
	import { Popover } from 'bits-ui';
	import { SmilePlus } from 'lucide-svelte';
	import { couch } from '$lib/features/couch/couch.svelte';
	import EmojiPicker from './EmojiPicker.svelte';
	import * as m from '$lib/paraglide/messages';

	// the couch's by default; Couch TV's room brings its own sender and quick picks
	let {
		portalTo,
		triggerClass = '',
		send = (emoji: string) => couch.sendEmoji(emoji),
		recent,
		autofocus = false
	}: {
		portalTo?: HTMLElement;
		triggerClass?: string;
		send?: (emoji: string) => void;
		recent?: string[];
		/** where a TV remote's focus lands (`data-tv-autofocus`) */
		autofocus?: boolean;
	} = $props();

	let open = $state(false);
	const quick = $derived(recent ?? couch.recentEmojis);

	function pick(emoji: string) {
		send(emoji);
		open = false;
	}
</script>

<div class="group relative flex flex-col items-center">
	<!-- recently-used emojis slide up on hover (or with focus on the button, for keyboards and
		remotes) for quick re-sending (same width) -->
	{#if quick.length > 0}
		<!-- pb (not mb) keeps the gap part of the hover region so moving onto the
			menu doesn't drop the hover and hide it before a click lands -->
		<div
			class="pointer-events-none absolute bottom-full flex w-full translate-y-2 flex-col gap-1.5 pb-2
				opacity-0 transition-all duration-200 group-focus-within:pointer-events-auto
				group-focus-within:translate-y-0 group-focus-within:opacity-100 group-hover:pointer-events-auto
				group-hover:translate-y-0 group-hover:opacity-100"
		>
			{#each quick as e (e)}
				<button
					class="flex aspect-square w-full items-center justify-center rounded-full border border-edge
						bg-surface-2/90 text-xl shadow-lg backdrop-blur transition-transform hover:scale-110
						focus-visible:scale-110"
					onclick={() => send(e)}
					aria-label={m.couch_react()}
				>
					{e}
				</button>
			{/each}
		</div>
	{/if}

	<Popover.Root bind:open>
		<Popover.Trigger
			class={triggerClass}
			aria-label={m.couch_react()}
			title={m.couch_react()}
			data-tv-autofocus={autofocus || undefined}
		>
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
