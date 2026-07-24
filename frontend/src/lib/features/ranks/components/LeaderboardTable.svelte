<script lang="ts">
	import { onMount } from 'svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { metricLabel, tierName } from '../labels';
	import { rankColor } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { LeaderRow, Metric } from '../types';

	let {
		rows,
		metric,
		value,
		display
	}: {
		rows: LeaderRow[];
		metric: Metric;
		value: (row: LeaderRow) => number;
		display: (n: number) => string;
	} = $props();

	const top = $derived(Math.max(1, ...rows.map(value)));

	let ready = $state(false);
	onMount(() => {
		ready = true;
	});
</script>

<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	<table class="w-full text-left text-sm">
		<caption class="sr-only">{m.leaderboard_table_caption({ metric: metricLabel(metric) })}</caption
		>
		<thead>
			<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
				<th scope="col" class="w-12 px-4 py-2.5 text-center">{m.leaderboard_col_rank()}</th>
				<th scope="col" class="px-4 py-2.5">{m.leaderboard_col_member()}</th>
				<th scope="col" class="hidden px-4 py-2.5 sm:table-cell">{m.leaderboard_col_level()}</th>
				<th scope="col" class="px-4 py-2.5 text-right">{m.leaderboard_col_value()}</th>
			</tr>
		</thead>
		<tbody>
			{#each rows as row, i (row.username)}
				<tr
					class="border-b border-edge/50 transition-colors last:border-0 hover:bg-surface-2/40
						{row.isSelf ? 'bg-accent-soft' : ''}"
				>
					<!-- the accent rule lives on the cell, not the row: an absolutely
					     positioned ::before on a <tr> becomes an anonymous table cell
					     and shifts every real cell across -->
					<td
						class="border-l-2 px-4 py-3 text-center font-semibold text-faint tnum
							{row.isSelf ? 'border-accent' : 'border-transparent'}"
					>
						{i + 1}
					</td>
					<td class="px-4 py-3">
						<div class="flex min-w-0 items-center gap-3">
							<UserAvatar
								name={row.displayName}
								avatarId={row.avatarId}
								seed={row.username}
								class="size-8 shrink-0 rounded-lg text-[10px]"
							/>
							<div class="min-w-0">
								<a
									href="/u/{row.username}"
									class="block truncate font-medium transition-colors hover:text-accent"
								>
									{row.displayName}
									{#if row.isSelf}
										<!-- the tint alone is never the only signal -->
										<span class="ml-1 text-[10px] font-normal text-faint">
											{m.leaderboard_you()}
										</span>
									{/if}
								</a>
								<span class="block truncate text-[11px] text-faint">@{row.username}</span>
							</div>
						</div>
					</td>
					<td class="hidden px-4 py-3 sm:table-cell">
						<span
							class="rounded px-1.5 py-0.5 text-[10px] font-bold whitespace-nowrap"
							style="background: color-mix(in srgb, {rankColor(row.tierCode)} 18%, transparent);
								color: {rankColor(row.tierCode)}"
						>
							{m.rank_level({ level: row.level })} · {tierName(row.tierCode)}
						</span>
					</td>
					<td class="px-4 py-3 text-right">
						<span class="font-semibold tnum">{display(value(row))}</span>
						<span class="mt-1 block h-1 overflow-hidden rounded-full bg-surface-2">
							<span
								class="block h-full origin-left rounded-full bg-accent transition-transform
									duration-700 ease-out"
								style="transform: scaleX({ready ? value(row) / top : 0});
									transition-delay: {Math.min(i * 40, 500)}ms"
							></span>
						</span>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
