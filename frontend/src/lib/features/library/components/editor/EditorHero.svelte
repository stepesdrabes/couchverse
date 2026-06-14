<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { ArrowLeft, ImagePlus, ListPlus, Sparkles, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { ArtworkRef, Title } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import Button from '$lib/components/ui/Button.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		title,
		artwork,
		busy = false,
		onFetchTmdb,
		onImportEpisodes,
		onDelete
	}: {
		title: Title;
		artwork: ArtworkRef[];
		busy?: boolean;
		onFetchTmdb: () => void;
		onImportEpisodes: () => void;
		onDelete: () => void;
	} = $props();

	const poster = $derived(artwork.find((a) => a.kind === 'poster'));
	const backdrop = $derived(artwork.find((a) => a.kind === 'backdrop'));

	let posterInput = $state<HTMLInputElement>();
	let backdropInput = $state<HTMLInputElement>();

	async function upload(kind: 'poster' | 'backdrop', files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			await libraryApi.uploadArtwork('title', title.id, kind, file);
			toast.success(kind === 'poster' ? m.library_poster_updated() : m.library_backdrop_updated());
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_upload_failed());
		}
	}

	async function remove(art: ArtworkRef) {
		try {
			await libraryApi.deleteArtwork(art.id);
			invalidateAll();
		} catch {
			toast.error(m.library_delete_artwork_failed());
		}
	}
</script>

<header class="group/hero relative mb-8 overflow-hidden">
	<div class="absolute inset-0">
		{#if backdrop}
			<img src={artworkUrl(backdrop.id)} alt="" class="size-full object-cover opacity-30" />
		{:else}
			<div class="size-full bg-gradient-to-br from-accent-soft/30 via-bg to-bg"></div>
		{/if}
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/70 to-bg/30"></div>
	</div>

	<!-- backdrop controls: hover-revealed on desktop, always visible on touch -->
	<div
		class="absolute top-5 right-5 z-10 flex gap-2 transition-opacity
			md:opacity-0 md:group-hover/hero:opacity-100 md:focus-within:opacity-100"
	>
		<button
			type="button"
			class="flex items-center gap-1.5 rounded-full border border-edge/60 bg-bg/60 px-3 py-1.5
				text-xs font-medium text-muted backdrop-blur transition-colors hover:text-text"
			onclick={() => backdropInput?.click()}
		>
			<ImagePlus class="size-3.5" />
			{backdrop ? m.library_replace_backdrop() : m.library_add_backdrop()}
		</button>
		{#if backdrop}
			<button
				type="button"
				class="rounded-full border border-edge/60 bg-bg/60 p-2 text-muted backdrop-blur
					transition-colors hover:text-danger"
				onclick={() => remove(backdrop)}
				aria-label={m.library_remove_backdrop()}
			>
				<Trash2 class="size-3.5" />
			</button>
		{/if}
	</div>

	<div class="relative mx-auto max-w-7xl px-8 pt-6 pb-8">
		<a
			href="/admin/library"
			class="mb-8 inline-flex items-center gap-1.5 text-xs font-medium text-faint transition-colors hover:text-text"
		>
			<ArrowLeft class="size-3.5" />
			{m.library_back_to_library()}
		</a>

		<div class="flex items-end gap-6">
			<div
				class="group/poster relative aspect-[2/3] w-24 shrink-0 overflow-hidden rounded-card
					border border-edge/60 bg-surface-2 shadow-2xl shadow-black/50 md:w-28"
			>
				{#if poster}
					<img src="{artworkUrl(poster.id)}?size=w342" alt="" class="size-full object-cover" />
				{:else}
					<span class="flex size-full items-center justify-center">
						<ImagePlus class="size-5 text-faint" />
					</span>
				{/if}
				<button
					type="button"
					class="absolute inset-0 flex items-center justify-center bg-black/55 opacity-0
						transition-opacity group-hover/poster:opacity-100 focus-visible:opacity-100"
					onclick={() => posterInput?.click()}
					title={poster ? m.library_replace_poster() : m.library_upload_poster()}
				>
					<ImagePlus class="size-5 text-white" />
				</button>
				{#if poster}
					<button
						type="button"
						class="absolute top-1.5 right-1.5 rounded-full bg-black/60 p-1.5 text-white/80 opacity-0
							transition-opacity group-hover/poster:opacity-100 hover:text-danger"
						onclick={() => remove(poster)}
						aria-label={m.library_remove_poster()}
					>
						<Trash2 class="size-3" />
					</button>
				{/if}
			</div>

			<div class="min-w-0 flex-1">
				<h1 class="truncate text-3xl font-extrabold tracking-tight md:text-4xl">{title.name}</h1>
				<div class="mt-3 flex flex-wrap items-center gap-3 text-xs text-muted">
					{#if title.year}<span class="tnum">{title.year}</span>{/if}
					<span class="capitalize">{title.kind}</span>
					<StatusPill status={title.status} />
				</div>

				<div class="mt-5 flex flex-wrap items-center gap-2">
					<Button variant="secondary" size="sm" disabled={busy} onclick={onFetchTmdb}>
						<Sparkles class="size-3.5" />
						{m.library_fetch_from_tmdb()}
					</Button>
					{#if title.kind === 'series'}
						<!-- span carries the hint: the disabled button swallows pointer events -->
						<span title={title.tmdbId ? undefined : m.library_link_tmdb_first()}>
							<Button
								variant="secondary"
								size="sm"
								disabled={!title.tmdbId || busy}
								onclick={onImportEpisodes}
							>
								<ListPlus class="size-3.5" />
								{m.library_import_episodes()}
							</Button>
						</span>
					{/if}
					<Button variant="danger" size="sm" onclick={onDelete}>
						<Trash2 class="size-3.5" />
						{m.library_delete_title()}
					</Button>
				</div>
			</div>
		</div>
	</div>
</header>

<input
	bind:this={posterInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		upload('poster', e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
<input
	bind:this={backdropInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		upload('backdrop', e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
