<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { Info, Radio, SkipBack, X } from 'lucide-svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Artwork from '$lib/features/catalog/components/Artwork.svelte';
	import { watchPath } from '$lib/features/playback/player';
	import type { CouchTvProgram } from '$lib/generated/core';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import { episodeLabel, fillBadge, timeRange } from '../channel';

	// A program's details from the guide: what it is, when it airs, and what can be done with
	// it now: tune in while it is on, watch it from the start once it has begun (members only:
	// the guide is theirs).
	let {
		program = $bindable(),
		day,
		now
	}: {
		/** the program shown; the sheet closes by setting it to undefined */
		program?: CouchTvProgram;
		/** its broadcast day's name */
		day: string;
		/** the wall clock, ms */
		now: number;
	} = $props();

	const lang = $derived(currentLang());
	const onNow = $derived(!!program && program.startsAtMs <= now && now < program.endsAtMs);
	const begun = $derived(!!program && program.startsAtMs <= now);
	const badge = $derived(fillBadge(program?.fill));
</script>

<Dialog.Root
	bind:open={
		() => !!program,
		(open) => {
			if (!open) program = undefined;
		}
	}
>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 animate-fade-in bg-black/60 backdrop-blur-sm" />
		<Dialog.Content
			class="fixed inset-x-0 bottom-0 z-50 max-h-[85dvh] animate-slide-up overflow-y-auto rounded-t-card
				border border-edge bg-surface-2 shadow-2xl shadow-black/60 scrollbar-none sm:inset-y-0
				sm:right-0 sm:left-auto sm:max-h-none sm:w-[26rem] sm:rounded-none sm:rounded-l-card"
		>
			{#if program}
				<div class="relative aspect-video w-full overflow-hidden bg-surface">
					<Artwork src={program.backdrop?.url ?? program.poster?.url} name={program.title ?? ''} />
					<div class="absolute inset-0 bg-gradient-to-t from-surface-2 via-transparent"></div>
					<Dialog.Close
						class="absolute top-3 right-3 rounded-full bg-black/55 p-2 text-white transition-colors
							hover:bg-black/75"
						aria-label={m.common_close()}
					>
						<X class="size-4" />
					</Dialog.Close>
				</div>

				<div class="space-y-4 p-6 pt-2">
					<div class="space-y-1.5">
						{#if onNow || badge}
							<div class="flex flex-wrap gap-1.5">
								{#if onNow}
									<span
										class="inline-flex items-center rounded bg-accent px-1.5 py-0.5 text-[10px] font-semibold
											tracking-wider text-[var(--color-on-accent)] uppercase"
									>
										{m.couchtv_on_air_now()}
									</span>
								{/if}
								{#if badge}
									<Badge>{badge}</Badge>
								{/if}
							</div>
						{/if}
						<Dialog.Title class="text-xl font-bold">{program.title}</Dialog.Title>
						{#if program.episode}
							<p class="text-sm text-muted">{episodeLabel(program.episode)}</p>
						{/if}
						<p class="flex gap-2 text-sm font-medium">
							<span>{day}</span>
							<span class="text-muted tnum">{timeRange(program, lang)}</span>
						</p>
					</div>

					{#if program.overview}
						<Dialog.Description class="text-sm leading-relaxed text-muted">
							{program.overview}
						</Dialog.Description>
					{/if}

					<div class="flex flex-wrap gap-2">
						{#if onNow}
							<a
								href="/tv"
								class="inline-flex h-10 items-center gap-2 rounded-full bg-accent px-5 text-sm font-semibold
									text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
								data-tv-autofocus
							>
								<Radio class="size-4" />
								{m.couchtv_tune_in()}
							</a>
						{/if}
						{#if begun && program.play}
							<a
								href={watchPath(program.play)}
								data-sveltekit-preload-data="tap"
								class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60 px-5
									text-sm font-semibold transition-colors hover:border-faint hover:bg-surface"
							>
								<SkipBack class="size-4" />
								{m.couchtv_watch_from_start()}
							</a>
						{/if}
						{#if program.titleSlug}
							<a
								href="/title/{program.titleSlug}"
								class="inline-flex h-10 items-center gap-2 rounded-full px-4 text-sm font-semibold text-muted
									transition-colors hover:bg-surface hover:text-text"
							>
								<Info class="size-4" />
								{m.couchtv_about_title()}
							</a>
						{/if}
					</div>
				</div>
			{/if}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
