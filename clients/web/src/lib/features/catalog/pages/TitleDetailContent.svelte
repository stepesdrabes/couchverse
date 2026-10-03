<script lang="ts">
	import { goto } from '$app/navigation';
	import { Check, Play, Plus, Shuffle } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import { TitleKind, type TitleDetailView } from '$lib/generated/core';
	import Artwork from '$lib/features/catalog/components/Artwork.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { paletteVars } from '$lib/theme';
	import { formatClock, formatRuntime } from '$lib/utils/format';
	import { qualityLabel } from '../labels';
	import * as m from '$lib/paraglide/messages';

	let { detail }: { detail: TitleDetailView } = $props();

	let seasonValue = $state('');
	// a logo that would not load gives way to the name
	let brokenLogo = $state<string>();

	const logo = $derived(detail.logo?.url === brokenLogo ? undefined : detail.logo);
	// the page takes its colours from the title's backdrop
	const accentStyle = $derived(detail.accent ? paletteVars(detail.accent) : '');
	const series = $derived(detail.kind === TitleKind.Series);
	const currentSeason = $derived(
		detail.seasons.find((s) => String(s.number) === seasonValue) ?? detail.seasons[0]
	);

	function play() {
		if (detail.play) goto(`/watch/${detail.play.target.kind}/${detail.play.target.id}`);
	}

	function playRandom() {
		const episodes = detail.seasons.flatMap((s) => s.episodes);
		if (!episodes.length) return;
		localStorage.setItem('cv.shuffle', '1'); // keep playing randomly in the player
		const episode = episodes[Math.floor(Math.random() * episodes.length)];
		goto(`/watch/episode/${episode.id}`);
	}

	const playLabel = $derived.by(() => {
		const action = detail.play;
		if (action?.resumeSeconds && !series) {
			return m.catalog_resume_from({ time: formatClock(action.resumeSeconds) });
		}
		if (action?.episode) {
			return m.catalog_play_episode({
				label: `S${action.episode.season} E${action.episode.episode}`
			});
		}
		return m.common_play();
	});
</script>

<svelte:head>
	<title>{m.catalog_title_page_title({ name: detail.name })}</title>
</svelte:head>

<div class="relative" style={accentStyle}>
	<div class="absolute inset-x-0 top-0 h-[480px] overflow-hidden">
		{#if detail.backdrop}
			<img src={detail.backdrop.url} alt="" class="size-full object-cover opacity-35" />
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
				<Artwork src={detail.poster?.url} name={detail.name} />
			</div>

			<div class="min-w-0 animate-slide-up">
				<h1 class="text-3xl font-extrabold tracking-tight md:text-5xl">
					{#if logo}
						<!-- the wordmark in the display language; its aspect holds the space until it loads -->
						<img
							src={logo.url}
							alt=""
							class="h-20 w-auto max-w-full object-contain object-left md:h-28"
							style:aspect-ratio={logo.aspect}
							onerror={() => (brokenLogo = logo.url)}
						/>
						<span class="sr-only">{detail.name}</span>
					{:else}
						{detail.name}
					{/if}
				</h1>

				<div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-muted">
					{#if detail.year}<span>{detail.year}</span>{/if}
					{#if series}
						<span>· {m.catalog_season_count({ count: detail.seasons.length })}</span>
					{:else if detail.runtimeMinutes}
						<span>· {formatRuntime(detail.runtimeMinutes)}</span>
					{/if}
					{#if detail.contentRating}
						<Badge>{detail.contentRating}</Badge>
					{/if}
					{#if detail.quality}
						<Badge>{qualityLabel(detail.quality)}</Badge>
					{/if}
					{#if detail.hdr}
						<Badge>{m.catalog_hdr()}</Badge>
					{/if}
				</div>

				{#if detail.overview}
					<p class="mt-4 max-w-2xl text-[15px] leading-relaxed text-muted">
						{detail.overview}
					</p>
				{/if}

				{#if detail.genres.length}
					<p class="mt-3 text-xs text-faint">{detail.genres.join(' · ')}</p>
				{/if}

				<div class="mt-6 flex items-center gap-3">
					<Button size="lg" onclick={play} disabled={!detail.play} data-tv-autofocus>
						<Play class="size-4 fill-current" />
						{playLabel}
					</Button>
					<Button
						variant="secondary"
						size="lg"
						onclick={() => catalog.setListed(detail.id, !detail.inList)}
						aria-pressed={detail.inList}
					>
						{#if detail.inList}
							<Check class="size-4" />
						{:else}
							<Plus class="size-4" />
						{/if}
						{m.nav_my_list()}
					</Button>
					{#if detail.shuffle}
						<Button variant="secondary" size="lg" onclick={playRandom}>
							<Shuffle class="size-4" />
							{m.catalog_random_episode()}
						</Button>
					{/if}
				</div>
			</div>
		</div>

		{#if series && detail.seasons.length > 0}
			<section class="mt-12">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="eyebrow">{m.catalog_episodes()}</h2>
					{#if detail.seasons.length > 1}
						<Select
							bind:value={seasonValue}
							label={m.catalog_season()}
							placeholder={String(currentSeason?.number ?? 1)}
							items={detail.seasons.map((s) => ({
								value: String(s.number),
								label: s.name || m.catalog_season_number({ number: s.number })
							}))}
						/>
					{/if}
				</div>

				<ul class="space-y-2">
					{#each currentSeason?.episodes ?? [] as ep, i (ep.id)}
						{@const name = ep.name || m.catalog_episode_number({ number: ep.number })}
						<li in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 40, 360) }}>
							<a
								href="/watch/episode/{ep.id}"
								data-sveltekit-preload-data="tap"
								class="group flex cursor-pointer gap-4 rounded-card border border-edge bg-surface/40 p-3
									transition-colors hover:border-accent/40 hover:bg-surface-2/60
									focus-visible:border-accent/40 focus-visible:bg-surface-2/60"
							>
								<div
									class="relative aspect-video w-32 shrink-0 overflow-hidden rounded-lg border
										border-edge/60 bg-surface-2 sm:w-44"
								>
									<Artwork src={ep.still?.url} {name} />
									<div
										class="absolute inset-0 flex items-center justify-center bg-black/45 opacity-0
											transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
									>
										<span
											class="rounded-full bg-accent p-2.5 text-[var(--color-on-accent)] shadow-lg"
										>
											<Play class="size-4 fill-current" />
										</span>
									</div>
									{#if ep.progress > 0}
										<div class="absolute inset-x-0 bottom-0 h-1 bg-black/50">
											<div class="h-full bg-accent" style="width: {ep.progress * 100}%"></div>
										</div>
									{/if}
								</div>

								<div class="min-w-0 flex-1 py-0.5">
									<div class="flex items-baseline gap-2">
										<span class="shrink-0 text-sm font-semibold text-faint tnum">{ep.number}</span>
										<p
											class="truncate text-sm font-semibold group-hover:text-accent group-focus-visible:text-accent"
										>
											{name}
										</p>
									</div>
									{#if ep.overview}
										<p class="mt-1 line-clamp-2 text-xs leading-relaxed text-faint">
											{ep.overview}
										</p>
									{/if}
								</div>

								<div class="shrink-0 py-0.5 text-right">
									<span class="text-xs text-faint tnum">
										{ep.durationSeconds ? formatClock(ep.durationSeconds) : ''}
									</span>
								</div>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/if}
	</div>
</div>
