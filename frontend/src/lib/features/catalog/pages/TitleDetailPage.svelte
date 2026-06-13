<script lang="ts">
	import { goto } from '$app/navigation';
	import { Check, Play, Plus } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import * as catalog from '$lib/features/catalog/api';
	import type { Episode, MediaFile } from '$lib/features/catalog/types';
	import Artwork from '$lib/features/catalog/components/Artwork.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { formatClock, formatRuntime, qualityLabel } from '$lib/utils/format';
	import { bannerAccent } from '$lib/utils/palette.svelte';

	let { data }: { data: Awaited<ReturnType<typeof catalog.getTitle>> } = $props();

	let listed = $state(data.inWatchlist);
	let seasonValue = $state('');

	const poster = $derived(data.artwork.find((a) => a.kind === 'poster'));
	const backdrop = $derived(data.artwork.find((a) => a.kind === 'backdrop'));

	// accent the whole page from the title's backdrop, like the home hero
	const accent = bannerAccent(() =>
		backdrop ? catalog.artworkUrl(backdrop.id, catalog.artworkVer(backdrop.createdAt)) : null
	);

	const fileByEpisode = $derived(
		new Map(data.mediaFiles.filter((f) => f.episodeId).map((f) => [f.episodeId as string, f]))
	);
	const maxHeight = $derived(Math.max(0, ...data.mediaFiles.map((f) => f.height)));
	const hdr = $derived(data.mediaFiles.some((f) => f.videoRange !== 'sdr'));

	// watch mode only surfaces episodes that actually have a playable file, and
	// drops seasons left empty by that filter
	const seasons = $derived(
		(data.seasons ?? [])
			.map((s) => ({ ...s, episodes: s.episodes.filter((ep) => fileByEpisode.has(ep.id)) }))
			.filter((s) => s.episodes.length > 0)
	);
	const currentSeason = $derived(
		seasons.find((s) => String(s.seasonNumber) === seasonValue) ?? seasons[0]
	);

	const movieResume = $derived(
		data.title.kind === 'movie' && (data.progress?.positionSeconds ?? 0) > 10
			? data.progress!.positionSeconds
			: 0
	);

	// first not-completed episode, for the series Play button
	const nextUp = $derived.by(() => {
		for (const season of seasons) {
			for (const ep of season.episodes) {
				const p = data.episodeProgress?.[ep.id];
				if (!p?.completed && fileByEpisode.has(ep.id)) return ep;
			}
		}
		return null;
	});

	function play() {
		if (data.title.kind === 'movie') goto(`/watch/movie/${data.title.id}`);
		else if (nextUp) goto(`/watch/episode/${nextUp.id}`);
	}

	async function toggleList() {
		try {
			if (listed) await catalog.removeFromList(data.title.id);
			else await catalog.addToList(data.title.id);
			listed = !listed;
		} catch {
			toast.error('Failed to update My List');
		}
	}

	const episodeProgressPct = (ep: Episode) => {
		const p = data.episodeProgress?.[ep.id];
		if (!p || p.durationSeconds === 0) return 0;
		if (p.completed) return 100;
		return (p.positionSeconds / p.durationSeconds) * 100;
	};

	const playable = (ep: Episode): MediaFile | undefined => fileByEpisode.get(ep.id);

	const nextUpLabel = $derived.by(() => {
		if (!nextUp) return '';
		const season = seasons.find((s) => s.id === nextUp.seasonId);
		return `S${season?.seasonNumber ?? 1} E${nextUp.episodeNumber}`;
	});
</script>

<svelte:head>
	<title>{data.title.name} - Couchverse</title>
</svelte:head>

<div class="relative" style={accent.style}>
	<div class="absolute inset-x-0 top-0 h-[480px] overflow-hidden">
		{#if backdrop}
			<img
				src={catalog.artworkUrl(backdrop.id, catalog.artworkVer(backdrop.createdAt))}
				alt=""
				class="size-full object-cover opacity-35"
			/>
		{:else}
			<div class="size-full bg-gradient-to-br from-accent-soft/40 via-bg to-bg"></div>
		{/if}
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/60 to-bg/30"></div>
	</div>

	<div class="relative mx-auto max-w-5xl px-6 pt-36 pb-16">
		<div class="flex flex-col gap-8 md:flex-row">
			<div
				class="hidden h-64 w-44 shrink-0 animate-slide-up overflow-hidden rounded-card border
					border-edge/60 shadow-2xl shadow-black/50 md:block"
			>
				<Artwork
					artworkId={poster?.id ?? null}
					v={poster ? catalog.artworkVer(poster.createdAt) : null}
					name={data.title.name}
				/>
			</div>

			<div class="min-w-0 animate-slide-up">
				<h1 class="text-3xl font-extrabold tracking-tight md:text-5xl">{data.title.name}</h1>

				<div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-muted">
					{#if data.title.year}<span>{data.title.year}</span>{/if}
					{#if data.title.kind === 'series'}
						<span>· {seasons.length} season{seasons.length === 1 ? '' : 's'}</span>
					{:else if data.title.runtimeMinutes}
						<span>· {formatRuntime(data.title.runtimeMinutes)}</span>
					{/if}
					{#if data.title.contentRating}
						<Badge>{data.title.contentRating}</Badge>
					{/if}
					{#if qualityLabel(maxHeight)}
						<Badge>{qualityLabel(maxHeight)}</Badge>
					{/if}
					{#if hdr}
						<Badge>HDR</Badge>
					{/if}
				</div>

				{#if data.title.overview}
					<p class="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
						{data.title.overview}
					</p>
				{/if}

				{#if data.title.genres.length}
					<p class="mt-3 text-xs text-faint">{data.title.genres.join(' · ')}</p>
				{/if}

				<div class="mt-6 flex items-center gap-3">
					<Button size="lg" onclick={play} disabled={data.title.kind === 'series' && !nextUp}>
						<Play class="size-4 fill-current" />
						{#if movieResume}
							Resume from {formatClock(movieResume)}
						{:else if data.title.kind === 'series' && nextUp}
							Play {nextUpLabel}
						{:else}
							Play
						{/if}
					</Button>
					<Button variant="secondary" size="lg" onclick={toggleList}>
						{#if listed}
							<Check class="size-4" />
						{:else}
							<Plus class="size-4" />
						{/if}
						My List
					</Button>
				</div>
			</div>
		</div>

		{#if data.title.kind === 'series' && seasons.length > 0}
			<section class="mt-12">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="eyebrow">Episodes</h2>
					{#if seasons.length > 1}
						<Select
							bind:value={seasonValue}
							label="Season"
							placeholder={String(currentSeason?.seasonNumber ?? 1)}
							items={seasons.map((s) => ({
								value: String(s.seasonNumber),
								label: s.name || `Season ${s.seasonNumber}`
							}))}
						/>
					{/if}
				</div>

				<ul class="space-y-2">
					{#each currentSeason?.episodes ?? [] as ep, i (ep.id)}
						{@const file = playable(ep)}
						{@const pct = episodeProgressPct(ep)}
						<li in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 40, 360) }}>
							<svelte:element
								this={file ? 'a' : 'div'}
								href={file ? `/watch/episode/${ep.id}` : undefined}
								class="group flex gap-4 rounded-card border border-edge bg-surface/40 p-3 transition-colors
									{file ? 'cursor-pointer hover:border-accent/40 hover:bg-surface-2/60' : 'opacity-50'}"
							>
								<div
									class="relative aspect-video w-32 shrink-0 overflow-hidden rounded-lg border
										border-edge/60 bg-surface-2 sm:w-44"
								>
									<Artwork
										artworkId={ep.thumbId ?? null}
										v={ep.thumbVer}
										name={ep.name || `Episode ${ep.episodeNumber}`}
									/>
									{#if file}
										<div
											class="absolute inset-0 flex items-center justify-center bg-black/45 opacity-0
												transition-opacity group-hover:opacity-100"
										>
											<span
												class="rounded-full bg-accent p-2.5 text-[var(--color-on-accent)] shadow-lg"
											>
												<Play class="size-4 fill-current" />
											</span>
										</div>
									{/if}
									{#if pct > 0}
										<div class="absolute inset-x-0 bottom-0 h-1 bg-black/50">
											<div class="h-full bg-accent" style="width: {pct}%"></div>
										</div>
									{/if}
								</div>

								<div class="min-w-0 flex-1 py-0.5">
									<div class="flex items-baseline gap-2">
										<span class="shrink-0 text-sm font-semibold text-faint tnum"
											>{ep.episodeNumber}</span
										>
										<p class="truncate text-sm font-semibold group-hover:text-accent">
											{ep.name || `Episode ${ep.episodeNumber}`}
										</p>
									</div>
									{#if ep.overview}
										<p class="mt-1 line-clamp-2 text-xs leading-relaxed text-faint">
											{ep.overview}
										</p>
									{/if}
								</div>

								<div class="shrink-0 py-0.5 text-right">
									{#if file}
										<span class="text-xs text-faint tnum">
											{file.durationSeconds ? formatClock(file.durationSeconds) : ''}
										</span>
									{:else}
										<span class="text-[11px] text-faint">no file</span>
									{/if}
								</div>
							</svelte:element>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	</div>
</div>
