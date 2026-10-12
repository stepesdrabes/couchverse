<script lang="ts">
	import { Popover } from 'bits-ui';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import EmojiReactionButton from '$lib/features/couch/components/EmojiReactionButton.svelte';
	import type { CouchTvView } from '$lib/generated/core';
	import * as m from '$lib/paraglide/messages';
	import { QUICK_REACTIONS, react } from '../channel';

	// Who is watching (each viewer once, guests under the name they typed) and the reaction
	// button, in the live player's control bar. The reaction button is the channel's main
	// control: a TV remote lands on it.
	let { view, portalTo }: { view: CouchTvView; portalTo?: HTMLElement } = $props();

	const faces = $derived(view.members.slice(0, 4));
	const unlisted = $derived(Math.max(0, view.viewers - view.members.length));
</script>

<div class="flex items-center gap-2">
	<EmojiReactionButton
		{portalTo}
		send={(emoji) => void react(emoji)}
		recent={QUICK_REACTIONS}
		autofocus
		triggerClass="player-btn"
	/>

	{#if view.viewers > 0}
		<Popover.Root>
			<Popover.Trigger
				class="flex items-center gap-2 rounded-full py-1 pr-1 pl-1 sm:pr-3 text-xs font-semibold text-white/85
					transition-colors hover:bg-white/10 hover:text-white focus-visible:bg-white/10"
				aria-label={m.couchtv_watching({ count: view.viewers })}
			>
				<span class="flex -space-x-2">
					{#each faces as member (member.id)}
						<UserAvatar
							name={member.displayName}
							src={member.avatar?.url}
							seed={member.seed}
							class="size-7 rounded-full text-[9px] ring-2 ring-black/70"
						/>
					{/each}
				</span>
				<!-- a phone's bar has room for the faces only -->
				<span class="hidden whitespace-nowrap tnum sm:inline">
					{m.couchtv_watching({ count: view.viewers })}
				</span>
			</Popover.Trigger>
			<Popover.Portal to={portalTo}>
				<Popover.Content
					side="top"
					sideOffset={10}
					class="z-50 max-h-[60vh] w-64 animate-pop-in overflow-y-auto rounded-card border border-edge
						bg-surface-2/95 p-2 shadow-xl backdrop-blur scrollbar-none"
				>
					<p class="px-2 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase">
						{m.couchtv_viewers()}
					</p>
					<ul class="space-y-0.5" aria-label={m.couchtv_viewers()}>
						{#each view.members as member (member.id)}
							<li class="flex items-center gap-2.5 rounded-lg px-2 py-1.5">
								<UserAvatar
									name={member.displayName}
									src={member.avatar?.url}
									seed={member.seed}
									class="size-7 rounded-lg text-[9px]"
								/>
								<span class="min-w-0 flex-1 truncate text-sm">{member.displayName}</span>
								{#if member.me}
									<span class="text-[10px] font-semibold text-accent-ink"
										>{m.couch_you_badge()}</span
									>
								{:else if member.guest}
									<span class="text-[10px] font-semibold text-muted">{m.couchtv_guest_badge()}</span
									>
								{/if}
							</li>
						{/each}
					</ul>
					{#if unlisted > 0}
						<p class="px-2 pt-1 text-xs text-muted tnum">+{unlisted}</p>
					{/if}
				</Popover.Content>
			</Popover.Portal>
		</Popover.Root>
	{/if}
</div>
