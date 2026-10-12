<script lang="ts">
	import { DISPLAY_FAMILY } from '@couchverse/broadcast/brand/wordmark';
	import { fade, fly } from 'svelte/transition';
	import type { CouchTvView } from '$lib/generated/core';
	import { colors } from '$lib/generated/tokens';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import { clock, episodeLabel, isProgram, timeRange } from '../channel';
	import Lockup from './Lockup.svelte';

	// The live overlays (D14) in the broadcast's own look, as the core cues them for what the
	// picture shows: over a program the corner bug and the Now and Next captions stay up, with
	// the controls or without; a break carries the channel's own graphics instead.
	let {
		view,
		at,
		active
	}: {
		view: CouchTvView;
		/** the picture's program date, run on to now */
		at?: number;
		/** the viewer is about: the controls are up, and the captions make room for them */
		active: boolean;
	} = $props();

	const lang = $derived(currentLang());
	const now = $derived(isProgram(view.now) ? view.now : undefined);
	const next = $derived(view.next);
	const style =
		`--brand: ${colors.accent}; --brand-ink: ${colors['accent-ink']};` +
		`--display: '${DISPLAY_FAMILY}', var(--font-sans);`;
</script>

<div class="pointer-events-none absolute inset-0 z-10" {style}>
	{#if view.cues.bug}
		<div
			transition:fade={{ duration: 400 }}
			class="bug absolute top-[4.5rem] right-4 flex items-center gap-3 sm:top-7 sm:right-8"
		>
			<Lockup size={22} />
			<span class="live flex items-center gap-1.5 rounded-sm px-1.5 py-0.5">
				<span class="dot size-1.5 rounded-full bg-white"></span>
				{m.couchtv_live()}
			</span>
			{#if at !== undefined}
				<span class="clock tnum">{clock(at, lang)}</span>
			{/if}
		</div>
	{/if}

	<div
		class="captions absolute flex flex-col items-start gap-1.5"
		style="bottom: {active ? '7.5rem' : '1.75rem'}; left: {active ? '4.5rem' : '1.75rem'}"
	>
		{#if view.cues.nowCaption && now}
			<div
				in:fly={{ x: -48, duration: 450 }}
				out:fade={{ duration: 300 }}
				class="caption flex max-w-[min(36rem,85vw)] items-stretch"
			>
				<span class="tag">{m.couchtv_now()}</span>
				<div class="panel flex min-w-0 items-baseline gap-3">
					<p class="title truncate">{now.title}</p>
					<p class="meta hidden shrink-0 gap-x-2 sm:flex">
						{#if now.episode}
							<span>{episodeLabel(now.episode)}</span>
						{/if}
						<span class="tnum">{timeRange(now, lang)}</span>
					</p>
				</div>
			</div>
		{/if}
		{#if view.cues.nextCaption && next}
			<div
				in:fly={{ x: -48, duration: 450 }}
				out:fade={{ duration: 300 }}
				class="caption flex max-w-[min(36rem,85vw)] items-stretch"
			>
				<span class="tag">{m.couchtv_next()}</span>
				<div class="panel flex min-w-0 items-baseline gap-3">
					<span class="time tnum">{clock(next.startsAtMs, lang)}</span>
					<p class="title truncate">{next.title}</p>
				</div>
			</div>
		{/if}
	</div>

	<!-- a screen reader hears what comes on, once per program -->
	<p class="sr-only" aria-live="polite">
		{now ? m.couchtv_now_announce({ title: now.title ?? '' }) : ''}
	</p>
</div>

<style lang="scss">
	.bug {
		filter: drop-shadow(0 1px 3px rgb(0 0 0 / 0.7));
	}

	.live,
	.clock {
		font-family: var(--display);
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-size: 0.8rem;
		color: white;
	}

	.live {
		background: var(--brand);
	}

	.dot {
		animation: breathe 2s ease-in-out infinite;
	}

	@keyframes breathe {
		50% {
			opacity: 0.35;
		}
	}

	/* they stay up through a program, so one line each; with the controls up they make room
	   for the quick reactions beside them */
	.captions {
		transition:
			bottom 0.25s ease,
			left 0.25s ease;
	}

	.caption {
		filter: drop-shadow(0 6px 18px rgb(0 0 0 / 0.55));
	}

	.tag {
		display: flex;
		align-items: center;
		padding: 0 0.75rem;
		background: var(--brand);
		color: white;
		font-family: var(--display);
		font-weight: 700;
		font-size: 0.8rem;
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.panel {
		padding: 0.45rem 0.8rem;
		background: rgb(0 0 0 / 0.82);
		color: white;
	}

	.title {
		font-family: var(--display);
		font-weight: 700;
		font-size: 1.1rem;
		line-height: 1.15;
		text-transform: uppercase;
	}

	.meta {
		font-size: 0.8rem;
		color: rgb(255 255 255 / 0.8);
	}

	.time {
		font-family: var(--display);
		font-weight: 700;
		font-size: 1.1rem;
		color: var(--brand-ink);
	}
</style>
