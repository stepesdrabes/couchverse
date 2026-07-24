<script lang="ts">
	import { KeyRound, Pencil } from 'lucide-svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Markdown from '$lib/components/ui/Markdown.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import { formatYearDate } from '$lib/utils/format';
	import { session } from '../session.svelte';
	import * as m from '$lib/paraglide/messages';

	// The identity card on its own, for servers with rankings switched off: same
	// banner/avatar/bio, none of the progression.
	let { onedit, onpassword }: { onedit: () => void; onpassword: () => void } = $props();
</script>

<div class="mx-auto max-w-2xl px-6 pt-28 pb-16">
	<div class="relative overflow-hidden rounded-card border border-edge bg-surface/40">
		{#if session.user?.bannerId}
			<div class="absolute inset-0" aria-hidden="true">
				<img
					src="{artworkUrl(session.user.bannerId)}?size=w780"
					alt=""
					class="size-full object-cover opacity-40"
				/>
				<div
					class="absolute inset-0 bg-gradient-to-t from-surface via-surface/85 to-surface/40"
				></div>
			</div>
		{/if}

		<div class="relative p-6 sm:p-8">
			<div class="flex items-center gap-5">
				<UserAvatar
					name={session.user?.displayName ?? '?'}
					avatarId={session.user?.avatarId}
					seed={session.user?.username}
					class="size-24 shrink-0 rounded-2xl text-3xl"
				/>
				<div class="min-w-0">
					<h1 class="truncate text-2xl font-extrabold tracking-tight">
						{session.user?.displayName}
					</h1>
					<p class="mt-1 text-sm text-faint">
						@{session.user?.username} · {session.user?.role === 'admin'
							? m.profile_role_admin()
							: m.profile_role_member()}
					</p>
					{#if session.user?.createdAt}
						<p class="text-xs text-faint">
							{m.profiles_joined({ date: formatYearDate(session.user.createdAt) })}
						</p>
					{/if}
				</div>
			</div>

			{#if session.user?.bio.trim()}
				<div class="mt-6 border-t border-edge/60 pt-5">
					<Markdown source={session.user.bio} />
				</div>
			{/if}

			<div class="mt-6 flex flex-wrap gap-2">
				<Button type="button" variant="secondary" size="sm" onclick={onedit}>
					<Pencil class="size-4" />
					{m.profiles_edit_profile()}
				</Button>
				<Button type="button" variant="ghost" size="sm" onclick={onpassword}>
					<KeyRound class="size-4" />
					{m.profile_password_heading()}
				</Button>
			</div>
		</div>
	</div>

	<p class="mt-4 text-xs text-faint">{m.profile_help_text()}</p>
</div>
