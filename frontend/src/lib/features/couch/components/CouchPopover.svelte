<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Popover } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { Copy, LogOut, Power } from 'lucide-svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import type { PlaybackKind } from '$lib/features/playback/api';
	import { couch } from '$lib/features/couch/couch.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		kind,
		id,
		portalTo,
		triggerClass = '',
		trigger
	}: {
		kind?: PlaybackKind;
		id?: string;
		portalTo?: HTMLElement;
		triggerClass?: string;
		trigger: Snippet;
	} = $props();

	let starting = $state(false);

	async function start() {
		if (!kind || !id) return;
		starting = true;
		try {
			await couch.startSession(kind, id);
		} catch {
			toast.error(m.couch_start_failed());
		} finally {
			starting = false;
		}
	}

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(couch.shareLink);
			toast.success(m.couch_link_copied());
		} catch {
			// clipboard blocked
		}
	}
</script>

<Popover.Root>
	<Popover.Trigger class={triggerClass} aria-label={m.couch_open()} title={m.couch_open()}>
		{@render trigger()}
	</Popover.Trigger>
	<Popover.Portal to={portalTo}>
		<Popover.Content
			side="top"
			sideOffset={10}
			class="z-50 w-72 animate-pop-in rounded-card border border-edge bg-surface-2/95 p-3 text-text shadow-xl backdrop-blur"
		>
			{#if !couch.active}
				<p class="px-1 text-sm font-semibold">{m.couch_start_session()}</p>
				<p class="mt-1 px-1 text-[11px] leading-snug text-faint">{m.couch_start_session_hint()}</p>
				{#if kind && id && session.user}
					<button
						class="mt-3 h-9 w-full rounded-full bg-accent text-sm font-semibold text-[var(--color-on-accent)]
							transition-colors hover:bg-accent-strong disabled:opacity-60"
						onclick={start}
						disabled={starting}
					>
						{starting ? m.couch_starting() : m.couch_start_session()}
					</button>
				{:else}
					<p class="mt-2 px-1 text-[11px] text-faint">{m.couch_start_from_video()}</p>
				{/if}
			{:else}
				<p class="eyebrow mb-1.5 px-1">{m.couch_share_label()}</p>
				<div class="flex gap-1.5">
					<input
						readonly
						value={couch.shareLink}
						class="h-8 min-w-0 flex-1 rounded-input border border-edge bg-surface px-2 text-xs text-muted"
					/>
					<button
						class="flex size-8 shrink-0 items-center justify-center rounded-input border border-edge
							text-muted transition-colors hover:bg-surface hover:text-text"
						onclick={copyLink}
						aria-label={m.couch_copy_link()}
						title={m.couch_copy_link()}
					>
						<Copy class="size-4" />
					</button>
				</div>

				<p class="eyebrow mt-3 mb-1 px-1">{m.couch_on_couch_count({ count: couch.count })}</p>
				<div class="max-h-44 space-y-0.5 overflow-y-auto scrollbar-none">
					{#each couch.participants as p (p.id)}
						<div class="flex items-center gap-2 rounded-lg px-1 py-1">
							<UserAvatar
								name={p.displayName}
								avatarId={p.avatarId ?? null}
								seed={p.seed}
								class="size-7 rounded-lg text-[10px]"
							/>
							<span class="min-w-0 flex-1 truncate text-xs">{p.displayName}</span>
							{#if p.isHost}
								<span
									class="rounded-full bg-accent/15 px-1.5 py-0.5 text-[10px] font-semibold text-accent"
								>
									{m.couch_host_badge()}
								</span>
							{/if}
							{#if p.id === couch.myParticipantId}
								<span class="text-[10px] font-medium text-faint">{m.couch_you_badge()}</span>
							{/if}
						</div>
					{/each}
				</div>

				<div class="mt-3 border-t border-edge/70 pt-2">
					{#if couch.isHost}
						<button
							class="flex h-8 w-full items-center justify-center gap-1.5 rounded-full bg-red-500/90
								text-xs font-semibold text-white transition-colors hover:bg-red-500"
							onclick={() => couch.end()}
						>
							<Power class="size-3.5" />
							{m.couch_end_session()}
						</button>
					{:else}
						<button
							class="flex h-8 w-full items-center justify-center gap-1.5 rounded-full border border-edge
								text-xs font-semibold text-muted transition-colors hover:bg-surface hover:text-text"
							onclick={() => couch.leave()}
						>
							<LogOut class="size-3.5" />
							{m.couch_leave()}
						</button>
					{/if}
				</div>
			{/if}
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
