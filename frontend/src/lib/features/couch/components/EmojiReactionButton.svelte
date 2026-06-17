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
