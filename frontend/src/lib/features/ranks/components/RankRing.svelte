<script lang="ts">
	import { onMount } from 'svelte';
	import { rankColor, rankGlow } from '../tiers';
	import type { TierCode } from '../types';

	let {
		tier,
		percent,
		size = 128,
		stroke = 7,
		orbit = false,
		pulse = 0,
		label = '',
		// the caller owns positioning: Tailwind resolves `relative` after
		// `absolute`, so baking one in here would silently beat an overlay class
		class: cls = 'relative'
	}: {
		tier: TierCode;
		percent: number;
		size?: number;
		stroke?: number;
		// a slow dashed halo that only spins while the card is hovered
		orbit?: boolean;
		// increment to replay the level-up flash; 0 keeps it quiet
		pulse?: number;
		label?: string;
		class?: string;
	} = $props();

	const gradientId = $props.id();
	const RADIUS = 44;
	const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

	const color = $derived(rankColor(tier));
	const glow = $derived(rankGlow(tier));
	const angle = $derived((Math.min(100, Math.max(0, percent)) / 100) * 2 * Math.PI);

	// Mount empty and fill on the next tick so the arc sweeps in rather than
	// snapping. `ready` also suppresses the level-up flash on first paint.
	let ready = $state(false);
	onMount(() => {
		ready = true;
	});

	const offset = $derived(CIRCUMFERENCE * (1 - (ready ? Math.min(100, percent) / 100 : 0)));
</script>

<span
	class="pointer-events-none inline-block {cls}"
	style="width: {size}px; height: {size}px"
	role="progressbar"
	aria-valuenow={Math.round(percent)}
	aria-valuemin="0"
	aria-valuemax="100"
	aria-label={label}
>
	<svg viewBox="0 0 100 100" class="size-full -rotate-90 overflow-visible">
		<defs>
			<linearGradient id={gradientId} x1="0" y1="0" x2="1" y2="1">
				<stop offset="0%" stop-color={color} stop-opacity="0.5" />
				<stop offset="100%" stop-color={color} />
			</linearGradient>
		</defs>

		{#if orbit}
			<circle
				cx="50"
				cy="50"
				r={RADIUS + 5}
				fill="none"
				stroke={color}
				stroke-opacity="0.35"
				stroke-width="1.5"
				stroke-dasharray="1 7"
				class="origin-center animate-spin [animation-duration:24s] [animation-play-state:paused]
					group-hover:[animation-play-state:running]"
			/>
		{/if}

		<circle
			cx="50"
			cy="50"
			r={RADIUS}
			fill="none"
			stroke="var(--color-edge)"
			stroke-width={stroke}
		/>
		<circle
			class="ring-arc"
			cx="50"
			cy="50"
			r={RADIUS}
			fill="none"
			stroke="url(#{gradientId})"
			stroke-width={stroke}
			stroke-linecap="round"
			stroke-dasharray={CIRCUMFERENCE}
			stroke-dashoffset={offset}
		/>

		{#if percent > 0}
			<circle
				class="ring-head"
				cx={50 + RADIUS * Math.cos(angle)}
				cy={50 + RADIUS * Math.sin(angle)}
				r={stroke * 0.42}
				fill={color}
				opacity={ready ? 1 : 0}
			/>
		{/if}
	</svg>

	{#if pulse > 0 && ready}
		{#key pulse}
			<span
				class="rank-pulse absolute inset-0 rounded-full"
				style="color: {color}; --pulse-glow: {glow}"
			></span>
		{/key}
	{/if}
</span>

<style lang="scss">
	// the app's slide-up easing, so the sweep feels like the rest of the UI
	.ring-arc {
		transition: stroke-dashoffset 900ms cubic-bezier(0.16, 1, 0.3, 1);
	}

	.ring-head {
		transition:
			cx 900ms cubic-bezier(0.16, 1, 0.3, 1),
			cy 900ms cubic-bezier(0.16, 1, 0.3, 1),
			opacity 400ms ease-out;
	}

	// A separate overlay rather than animating the ring itself, which would
	// restart the dash-offset sweep every time the level changes. box-shadow on a
	// scaled element beats filter: drop-shadow, which repaints every frame.
	.rank-pulse {
		box-shadow:
			0 0 0 2px currentColor,
			0 0 14px var(--pulse-glow);
		animation: rank-pulse 700ms cubic-bezier(0.16, 1, 0.3, 1) both;
	}

	@keyframes rank-pulse {
		from {
			transform: scale(0.92);
			opacity: 0.9;
		}
		to {
			transform: scale(1.55);
			opacity: 0;
		}
	}
</style>
