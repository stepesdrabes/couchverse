<script lang="ts">
	import { Loader, Play } from 'lucide-svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import type { CouchInfo } from '$lib/features/couch/api';
	import { accentVars } from '$lib/theme';
	import * as m from '$lib/paraglide/messages';

	let {
		info,
		joining,
		error,
		onstart
	}: { info: CouchInfo; joining: boolean; error: boolean; onstart: () => void } = $props();

	const accentStyle = $derived(
		info.display?.backdropAccent ? accentVars(info.display.backdropAccent) : ''
	);
</script>

<div
	class="relative flex h-dvh w-full items-center justify-center overflow-hidden bg-black"
	style={accentStyle}
>
	{#if info.display?.backdropId}
		<img
			src="{artworkUrl(info.display.backdropId)}?size=w780"
			alt=""
			class="absolute inset-0 size-full scale-105 object-cover opacity-30 blur-xl"
		/>
		<div class="absolute inset-0 bg-gradient-to-t from-black via-black/80 to-black/60"></div>
	{/if}

	<div class="relative z-10 flex max-w-md flex-col items-center gap-5 px-6 text-center">
		<UserAvatar
			name={info.hostName}
			avatarId={info.hostAvatarId}
			seed={info.hostSeed}
			class="size-20 rounded-2xl text-2xl shadow-xl"
		/>
		<div class="space-y-1">
			<p class="text-sm text-muted">{m.couch_join_heading({ name: info.hostName })}</p>
			{#if info.display}
				<h1 class="text-2xl font-bold text-white">{info.display.title}</h1>
				{#if info.display.subtitle}
					<p class="text-sm text-white/70">{info.display.subtitle}</p>
				{/if}
			{:else}
				<h1 class="text-xl font-semibold text-white">
					{m.couch_host_choosing({ name: info.hostName })}
				</h1>
			{/if}
		</div>

		<button
			class="mt-2 inline-flex h-12 items-center justify-center gap-2 rounded-full bg-accent px-8 text-sm
				font-semibold text-[var(--color-on-accent)] shadow-lg transition-colors hover:bg-accent-strong
				disabled:opacity-70"
			onclick={onstart}
			disabled={joining}
		>
			{#if joining}
				<Loader class="size-4 animate-spin" />
			{:else}
				<Play class="size-4 fill-current" />
			{/if}
			{m.couch_join_start()}
		</button>

		{#if error}
			<p class="text-sm text-danger">{m.couch_join_failed()}</p>
		{:else if info.participants > 0}
			<p class="text-xs text-faint">{m.couch_on_couch_count({ count: info.participants })}</p>
		{/if}
	</div>
</div>
