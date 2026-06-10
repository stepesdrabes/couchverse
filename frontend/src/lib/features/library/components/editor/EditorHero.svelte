<script lang="ts">
	import { ArrowLeft, ListPlus, Sparkles, Trash2 } from 'lucide-svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { ArtworkRef, Title } from '$lib/features/catalog/types';
	import Button from '$lib/components/ui/Button.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';

	let {
		title,
		artwork,
		onFetchTmdb,
		onImportEpisodes,
		onDelete
	}: {
		title: Title;
		artwork: ArtworkRef[];
		onFetchTmdb: () => void;
		onImportEpisodes: () => void;
		onDelete: () => void;
	} = $props();

	const poster = $derived(artwork.find((a) => a.kind === 'poster'));
	const backdrop = $derived(artwork.find((a) => a.kind === 'backdrop'));
</script>

<header class="relative -mx-8 mb-8 overflow-hidden">
	<div class="absolute inset-0">
		{#if backdrop}
			<img src={artworkUrl(backdrop.id)} alt="" class="size-full object-cover opacity-30" />
		{:else}
			<div class="size-full bg-gradient-to-br from-accent-soft/30 via-bg to-bg"></div>
		{/if}
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/70 to-bg/30"></div>
	</div>

	<div class="relative px-8 pt-6 pb-8">
		<a
			href="/admin/library"
			class="mb-8 inline-flex items-center gap-1.5 text-xs font-medium text-faint transition-colors hover:text-text"
		>
			<ArrowLeft class="size-3.5" />
			Library
		</a>

		<div class="flex items-end gap-6">
			<div
				class="hidden aspect-[2/3] w-28 shrink-0 overflow-hidden rounded-card border border-edge/60
					bg-surface-2 shadow-2xl shadow-black/50 md:block"
			>
				{#if poster}
					<img src="{artworkUrl(poster.id)}?size=w342" alt="" class="size-full object-cover" />
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
					<Button variant="secondary" size="sm" onclick={onFetchTmdb}>
						<Sparkles class="size-3.5" />
						Fetch from TMDB
					</Button>
					{#if title.kind === 'series'}
						<!-- span carries the hint: the disabled button swallows pointer events -->
						<span title={title.tmdbId ? undefined : 'Link the show via “Fetch from TMDB” first'}>
							<Button
								variant="secondary"
								size="sm"
								disabled={!title.tmdbId}
								onclick={onImportEpisodes}
							>
								<ListPlus class="size-3.5" />
								Import episodes
							</Button>
						</span>
					{/if}
					<Button variant="danger" size="sm" onclick={onDelete}>
						<Trash2 class="size-3.5" />
						Delete title
					</Button>
				</div>
			</div>
		</div>
	</div>
</header>
