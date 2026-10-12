<script lang="ts">
	import { Radio } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { useScreen } from '$lib/core/screen.svelte';
	import type { CouchTvOnNowView } from '$lib/generated/core';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import {
		clock,
		COUCH_TV_ON_NOW,
		episodeLabel,
		isProgram,
		progressAt,
		timeRange
	} from '../channel';
	import Lockup from './Lockup.svelte';

	// Home's "On now" card: what Couch TV airs, how far it has come and what follows, with a
	// way in. The core asks again when what is on ends; until its first answer the card takes
	// no room, so home never waits for the channel.
	const card = useScreen<CouchTvOnNowView>(() => COUCH_TV_ON_NOW);

	const lang = $derived(currentLang());
	// a break between programs is on air too: the card then leads with what comes next
	const onAir = $derived(!!card.view?.now);
	const program = $derived(isProgram(card.view?.now) ? card.view.now : undefined);
	const next = $derived(card.view?.next);
	const art = $derived(program?.backdrop ?? next?.backdrop);

	let wall = $state(Date.now());
	onMount(() => {
		const timer = setInterval(() => (wall = Date.now()), 30_000);
		return () => clearInterval(timer);
	});
</script>

{#if onAir || next}
	<!-- pt-12: the home's rows are pulled up over the hero's fade; the card clears its controls -->
	<section class="px-6 pt-12 lg:px-12" aria-labelledby="couchtv-on-now">
		<div
			class="relative flex min-h-44 overflow-hidden rounded-card border border-edge/60 bg-surface sm:min-h-52"
		>
			{#if art}
				<img src={art.url} alt="" class="absolute inset-0 size-full object-cover opacity-45" />
			{/if}
			<div class="absolute inset-0 bg-gradient-to-r from-bg via-bg/85 to-bg/10"></div>

			<div class="relative flex max-w-2xl flex-col justify-center gap-3 p-6 sm:p-8">
				<div class="flex items-center gap-3">
					<Lockup size={18} />
					<h2 id="couchtv-on-now" class="eyebrow">
						{onAir ? m.couchtv_on_now() : m.couchtv_off_air()}
					</h2>
				</div>

				{#if program}
					<div>
						<p class="text-2xl font-bold sm:text-3xl">{program.title}</p>
						<p class="mt-1 flex flex-wrap gap-x-3 text-sm text-muted">
							{#if program.episode}
								<span>{episodeLabel(program.episode)}</span>
							{/if}
							<span class="tnum">{timeRange(program, lang)}</span>
						</p>
					</div>
					<div
						class="h-1 w-full max-w-sm overflow-hidden rounded-full bg-white/15"
						role="progressbar"
						aria-label={m.player_live_progress()}
						aria-valuemin={0}
						aria-valuemax={100}
						aria-valuenow={Math.round(progressAt(program, wall) * 100)}
					>
						<div
							class="h-full rounded-full bg-accent"
							style="width: {progressAt(program, wall) * 100}%"
						></div>
					</div>
				{/if}

				<div class="flex flex-wrap items-center gap-x-5 gap-y-2">
					<a
						href="/tv"
						class="inline-flex h-10 items-center gap-2 rounded-full bg-accent px-5 text-sm font-semibold
							text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
					>
						<Radio class="size-4" />
						{m.couchtv_tune_in()}
					</a>
					{#if next}
						<p class="text-sm">
							<span class="font-semibold text-muted uppercase">{m.couchtv_next()}</span>
							<span class="ml-1.5 font-semibold tnum">{clock(next.startsAtMs, lang)}</span>
							<span class="ml-1.5">{next.title}</span>
						</p>
					{/if}
				</div>
			</div>
		</div>
	</section>
{/if}
