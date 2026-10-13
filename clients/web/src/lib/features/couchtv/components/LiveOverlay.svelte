<script lang="ts">
	import { BRAND } from '@couchverse/broadcast/brand/wordmark';
	import { expoOut } from 'svelte/easing';
	import { fade, fly } from 'svelte/transition';
	import type { CouchTvView } from '$lib/generated/core';
	import { colors } from '$lib/generated/tokens';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import { clock, episodeLabel, isProgram, timeRange } from '../channel';
	import Lockup from './Lockup.svelte';

	// The live overlays (D14) in the broadcast's own look, as the core cues them for what the
	// picture shows: over a program the corner bug stays up all the time, the Now and Next
	// captions come with the controls; a break carries the channel's own graphics instead. The
	// bug is the broadcast's, drawn where a break burns its own into the picture (its right edge
	// 5 % of the frame's width in, its top 5.93 % down, 3.7 % of its height tall, on the stream's
	// 16:9 frame as the player contains it, over the broadcast's corner shade), so the cut between
	// a program and a break never moves it. Sharp and flat (D42): square marks, no shadows; shades
	// keep the type legible over the picture.
	let {
		view,
		at,
		active
	}: {
		view: CouchTvView;
		/** the picture's program date, run on to now */
		at?: number;
		/** the viewer is about: the controls are up */
		active: boolean;
	} = $props();

	const lang = $derived(currentLang());
	const now = $derived(isProgram(view.now) ? view.now : undefined);
	const next = $derived(view.next);
	const captioned = $derived(
		active && ((view.cues.nowCaption && !!now) || (view.cues.nextCaption && !!next))
	);
	// the brand's red and paper, whatever the site's accent
	const style = `--brand: ${colors.accent}; --brand-ink: ${colors['accent-ink']}; --paper: ${BRAND.paper};`;
</script>

<div class="pointer-events-none absolute inset-0 z-10" {style}>
	{#if view.cues.bug}
		<div class="frame" transition:fade={{ duration: 400 }}>
			<div class="bug-scrim"></div>
			<div class="bug">
				<Lockup size={34} class="bug-lockup" />
				<span class="rule" aria-hidden="true"></span>
				<span class="live">
					<span class="dot" aria-hidden="true"></span>
					{m.couchtv_live()}
				</span>
				{#if at !== undefined}
					<span class="clock tnum">{clock(at, lang)}</span>
				{/if}
			</div>
		</div>
	{/if}

	{#if captioned}
		<div class="caption-scrim" transition:fade={{ duration: 300 }}></div>
	{/if}

	<!-- above the bar, aside for the quick reactions -->
	<div class="absolute bottom-[7.5rem] left-[4.5rem] flex flex-col items-start gap-5">
		{#if active && view.cues.nowCaption && now}
			<div
				in:fly={{ x: -48, duration: 450, easing: expoOut }}
				out:fade={{ duration: 300 }}
				class="caption min-w-0 max-w-[min(40rem,85vw)]"
			>
				<span class="label">{m.couchtv_now()}</span>
				<p class="title truncate">{now.title}</p>
				<p class="meta flex flex-wrap gap-x-2">
					{#if now.episode}
						<span>{episodeLabel(now.episode)}</span>
					{/if}
					<span class="tnum">{timeRange(now, lang)}</span>
				</p>
			</div>
		{/if}
		{#if active && view.cues.nextCaption && next}
			<div
				in:fly={{ x: -48, duration: 450, easing: expoOut }}
				out:fade={{ duration: 300 }}
				class="caption min-w-0 max-w-[min(40rem,85vw)]"
			>
				<span class="label">{m.couchtv_next()}</span>
				<div class="flex min-w-0 items-baseline gap-3">
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
	// the stream's 16:9 frame as the video shows it, contained and centred: the live player
	// fills the viewport (viewport units rather than container queries, which TV browsers may
	// lack)
	.frame {
		// the broadcast's bug, measured in its own 40 px: 3.7 % of the frame's height, at least 20 px
		--u: calc(max(20px, min(3.7vh, 2.08vw)) / 40);
		position: absolute;
		left: 50%;
		top: 50%;
		width: min(100vw, 177.78vh);
		height: min(100vh, 56.25vw);
		transform: translate(-50%, -50%);
	}

	// the broadcast's corner shade, in the bug's units so it covers a small bug too
	.bug-scrim {
		position: absolute;
		top: 0;
		right: 0;
		width: calc(var(--u) * 760);
		height: calc(var(--u) * 300);
		background: radial-gradient(
			100% 100% at 100% 0%,
			rgb(5 5 5 / 0.6),
			rgb(5 5 5 / 0.33) 45%,
			transparent
		);
	}

	.bug {
		position: absolute;
		top: 5.93%;
		right: 5%;
		display: flex;
		align-items: center;
		gap: calc(var(--u) * 18);
		height: calc(var(--u) * 40);
		color: var(--paper);
		white-space: nowrap;
	}

	.bug :global(.bug-lockup svg) {
		width: auto;
		height: calc(var(--u) * 36);
	}

	.rule {
		width: calc(var(--u) * 2);
		height: calc(var(--u) * 26);
		background: rgb(244 237 228 / 0.24);
	}

	.live {
		display: flex;
		align-items: center;
		gap: calc(var(--u) * 8);
		// the tracking after the last letter is not a gap
		margin-right: -0.12em;
		font-size: calc(var(--u) * 19);
		font-weight: 800;
		line-height: 1;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	// square and steady, as on air
	.dot {
		width: calc(var(--u) * 10);
		height: calc(var(--u) * 10);
		background: var(--brand);
	}

	.clock {
		font-size: calc(var(--u) * 28);
		font-weight: 700;
		line-height: 1;
		letter-spacing: -0.01em;
	}

	// the captions' corner, shaded as the broadcast shades its tag's
	.caption-scrim {
		position: absolute;
		left: 0;
		bottom: 0;
		width: min(72rem, 100%);
		height: min(34rem, 80%);
		background: radial-gradient(
			100% 100% at 0% 100%,
			rgb(5 5 5 / 0.75),
			rgb(5 5 5 / 0.45) 45%,
			transparent
		);
	}

	.caption {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		color: var(--paper);
	}

	.label {
		font-size: 0.95rem;
		font-weight: 700;
		color: var(--brand-ink);
	}

	.title {
		font-weight: 800;
		font-size: 1.75rem;
		line-height: 1.15;
		letter-spacing: -0.02em;
	}

	.meta {
		font-size: 0.9rem;
		color: rgb(244 237 228 / 0.72);
	}

	.time {
		font-weight: 800;
		font-size: 1.75rem;
		letter-spacing: -0.02em;
		color: var(--brand-ink);
	}
</style>
