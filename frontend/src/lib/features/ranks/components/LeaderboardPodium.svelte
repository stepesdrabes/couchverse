<script lang="ts">
	import { scale } from 'svelte/transition';
	import { backOut } from 'svelte/easing';
	import { onMount } from 'svelte';
	import { Crown } from 'lucide-svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import CountUp from './CountUp.svelte';
	import { PODIUM } from '../tiers';
	import type { LeaderRow } from '../types';

	let {
		rows,
		value,
		display
	}: {
		// exactly three, already sorted
		rows: LeaderRow[];
		value: (row: LeaderRow) => number;
		display: (n: number) => string;
	} = $props();

	// DOM order is 1-2-3 so a screen reader reads the actual ranking; the visual
	// 2-1-3 arrangement is pure `order-*`.
	const LAYOUT = [
		{ order: 'order-2', height: 'h-28', avatar: 'size-16 sm:size-20', delay: 200 },
		{ order: 'order-1', height: 'h-20', avatar: 'size-14 sm:size-16', delay: 90 },
		{ order: 'order-3', height: 'h-16', avatar: 'size-14 sm:size-16', delay: 0 }
	];

	// The crown lands after the plinths have settled, so it reads as the payoff.
	let crowned = $state(false);
	onMount(() => {
		const t = setTimeout(() => (crowned = true), 700);
		return () => clearTimeout(t);
	});

	// Modelled on the couch's growX: a css transition, so the global
	// reduced-motion clamp collapses it without a second code path.
	function riseY(_node: Element, { duration = 620, delay = 0 } = {}) {
		return {
			duration,
			delay,
			easing: backOut,
			css: (t: number) => `transform-origin: bottom; transform: scaleY(${t}); opacity: ${t};`
		};
	}
</script>

<div class="mb-10 flex items-end justify-center gap-4 sm:gap-8">
	{#each rows as row, i (row.username)}
		{@const metal = PODIUM[i]}
		{@const layout = LAYOUT[i]}
		<div class="flex w-24 flex-col items-center sm:w-32 {layout.order}">
			{#if i === 0}
				<span class="mb-1 h-5">
					{#if crowned}
						<span in:scale={{ duration: 320, start: 0.4, easing: backOut }}>
							<Crown class="size-5" style="color: {metal.ring}" />
						</span>
					{/if}
				</span>
			{/if}

			<a href="/u/{row.username}" class="flex min-w-0 flex-col items-center">
				<span
					class="rounded-2xl p-0.5 transition-transform hover:scale-105"
					style="background: linear-gradient(140deg, {metal.from}, {metal.to})"
				>
					<UserAvatar
						name={row.displayName}
						avatarId={row.avatarId}
						seed={row.username}
						class="{layout.avatar} rounded-[14px] text-lg"
					/>
				</span>
				<p class="mt-2 max-w-full truncate text-sm font-semibold">{row.displayName}</p>
				<p class="max-w-full truncate text-[11px] text-faint">@{row.username}</p>
			</a>

			<p class="text-lg font-extrabold" style="color: {metal.ring}">
				<CountUp value={value(row)} format={display} />
			</p>

			<div
				class="mt-3 flex w-full items-start justify-center rounded-t-xl border-t-2 pt-2 {layout.height}"
				style="border-color: {metal.ring};
					background: linear-gradient(180deg, {metal.from}22, transparent)"
				in:riseY={{ delay: layout.delay }}
			>
				<span class="text-2xl font-extrabold text-faint/60 tnum">{i + 1}</span>
			</div>
		</div>
	{/each}
</div>
