<script lang="ts">
	import { achievementIcon, achievementName } from '../labels';
	import { ACHIEVEMENT_TIERS } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { Achievement } from '../types';

	let {
		achievement,
		size = 'md',
		celebrate = false,
		onselect = undefined
	}: {
		achievement: Achievement;
		size?: 'sm' | 'md' | 'lg';
		// plays the unlock animation; only the celebration surfaces set this
		celebrate?: boolean;
		onselect?: (achievement: Achievement) => void;
	} = $props();

	const SIZES = { sm: 'size-10', md: 'size-16', lg: 'size-20' } as const;
	const metal = $derived(ACHIEVEMENT_TIERS[achievement.tier] ?? ACHIEVEMENT_TIERS.bronze);
	const Icon = $derived(achievementIcon(achievement.code));
	const unlocked = $derived(achievement.unlocked);
	const showRing = $derived(!unlocked && achievement.target > 1);

	const RADIUS = 46;
	const CIRCUMFERENCE = 2 * Math.PI * RADIUS;
	const offset = $derived(CIRCUMFERENCE * (1 - achievement.percent / 100));

	// Deterministic per-index jitter: a spark burst that is identical on every
	// render, so no Math.random() runs during the animation.
	const SPARKS = [0, 1, 2, 3, 4, 5, 6, 7].map((i) => ({
		angle: i * 45 + [6, -9, 4, -5, 11, -7, 3, -11][i],
		distance: 26 + [4, 9, 0, 7, 2, 11, 5, 8][i]
	}));

	const ariaLabel = $derived(
		`${achievementName(achievement.code)}. ` +
			(unlocked
				? m.achievement_unlocked()
				: m.achievement_progress({ current: achievement.value, target: achievement.target }))
	);
</script>

<svelte:element
	this={onselect ? 'button' : 'span'}
	type={onselect ? 'button' : undefined}
	class="relative inline-flex shrink-0 items-center justify-center {SIZES[size]}"
	style="--tier-from: {metal.from}; --tier-to: {metal.to};
		--tier-ring: {metal.ring}; --tier-glow: {metal.glow}"
	aria-label={onselect ? ariaLabel : undefined}
	role={onselect ? undefined : 'img'}
	onclick={onselect ? () => onselect(achievement) : undefined}
>
	{#if showRing}
		<svg viewBox="0 0 100 100" class="absolute inset-0 -rotate-90" aria-hidden="true">
			<circle cx="50" cy="50" r={RADIUS} fill="none" stroke="var(--color-edge)" stroke-width="5" />
			<circle
				cx="50"
				cy="50"
				r={RADIUS}
				fill="none"
				stroke="var(--color-faint)"
				stroke-width="5"
				stroke-linecap="round"
				stroke-dasharray={CIRCUMFERENCE}
				stroke-dashoffset={offset}
			/>
		</svg>
	{/if}

	<span
		class="badge relative flex size-[82%] items-center justify-center rounded-full"
		class:unlocked
	>
		<Icon class="size-1/2" />
		{#if celebrate}
			<span class="shock" aria-hidden="true"></span>
			{#each SPARKS as spark, i (i)}
				<span
					class="spark"
					style="--a: {spark.angle}deg; --d: {spark.distance}px"
					aria-hidden="true"
				></span>
			{/each}
		{/if}
	</span>
</svelte:element>

<style lang="scss">
	.badge {
		border: 1px solid var(--color-edge);
		background: var(--color-surface-2);
		// muted, not faint: a 3.2:1 icon is unreadable rather than merely quiet
		color: var(--color-muted);
		transition:
			transform 200ms cubic-bezier(0.16, 1, 0.3, 1),
			border-color 200ms;

		&.unlocked {
			background: linear-gradient(140deg, var(--tier-from), var(--tier-to));
			border-color: color-mix(in srgb, var(--tier-from) 55%, #000);
			box-shadow:
				inset 0 1px 0 rgb(255 255 255 / 0.35),
				0 2px 12px -3px var(--tier-glow);
			// the dark on-accent value the rest of the app uses
			color: #0b0c10;
			overflow: hidden;
		}
	}

	button {
		cursor: pointer;

		&:hover .badge.unlocked {
			transform: scale(1.06);
		}

		&:hover .badge:not(.unlocked) {
			border-color: var(--color-faint);
		}

		// the app's press grammar
		&:active .badge {
			transform: scale(0.97);
		}
	}

	// The celebration. Four scoped keyframes, twelve animated elements, all
	// opacity/transform, about 1.2s, once - no confetti library and no rAF.
	:global(.achievement-celebrate) .badge {
		animation: badge-pop 620ms cubic-bezier(0.34, 1.56, 0.64, 1) 120ms both;

		&.unlocked::after {
			content: '';
			position: absolute;
			inset: -30%;
			background: linear-gradient(
				105deg,
				transparent 38%,
				rgb(255 255 255 / 0.85) 50%,
				transparent 62%
			);
			mix-blend-mode: overlay;
			transform: translateX(-130%);
			animation: badge-shine 900ms ease-out 520ms;
		}
	}

	@keyframes badge-pop {
		0% {
			transform: scale(0.2) rotate(-18deg);
			opacity: 0;
		}
		60% {
			transform: scale(1.12) rotate(4deg);
			opacity: 1;
		}
		100% {
			transform: scale(1) rotate(0deg);
			opacity: 1;
		}
	}

	@keyframes badge-shine {
		to {
			transform: translateX(130%);
		}
	}

	.shock {
		position: absolute;
		inset: 0;
		border-radius: 9999px;
		border: 2px solid var(--tier-ring);
		animation: badge-shock 700ms cubic-bezier(0.16, 1, 0.3, 1) 300ms both;
	}

	@keyframes badge-shock {
		from {
			transform: scale(0.7);
			opacity: 0.85;
		}
		to {
			transform: scale(2.1);
			opacity: 0;
		}
	}

	.spark {
		position: absolute;
		left: 50%;
		top: 50%;
		width: 3px;
		height: 3px;
		border-radius: 9999px;
		background: var(--tier-ring);
		animation: badge-spark 800ms cubic-bezier(0.2, 0.7, 0.3, 1) 320ms both;
	}

	@keyframes badge-spark {
		0% {
			transform: rotate(var(--a)) translateY(0) scale(0.4);
			opacity: 0;
		}
		25% {
			opacity: 1;
		}
		100% {
			transform: rotate(var(--a)) translateY(calc(var(--d) * -1)) scale(0);
			opacity: 0;
		}
	}

	// display:none rather than leaning on the global duration clamp, so these
	// twelve elements are never laid out or painted at all
	@media (prefers-reduced-motion: reduce) {
		.spark,
		.shock {
			display: none;
		}

		:global(.achievement-celebrate) .badge.unlocked::after {
			display: none;
		}
	}
</style>
