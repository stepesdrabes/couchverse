<script lang="ts">
	import { AlertTriangle, Clapperboard, Gauge, HardDrive, Hourglass } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import RankedList from '$lib/components/ui/RankedList.svelte';
	import SegmentBar, { type Segment } from '$lib/components/ui/SegmentBar.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import StatTile from '$lib/components/ui/StatTile.svelte';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { formatBytes, formatEta, formatRelative } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
	import { adminCouchTvRenderHealth, type CouchTvRenderHealth } from '../api';
	import { renderColor } from '../labels';

	let health = $state<CouchTvRenderHealth | null>(null);
	let failed = $state(false);
	let now = $state(Date.now());
	const lang = $derived(currentLang());

	async function poll() {
		if (document.visibilityState === 'hidden' && health) return;
		try {
			health = await adminCouchTvRenderHealth();
			now = Date.now();
			failed = false;
		} catch {
			failed = true;
		}
	}

	onMount(() => {
		poll();
		const t = setInterval(poll, 10_000);
		return () => clearInterval(t);
	});

	const q = $derived(health?.queue);
	const b = $derived(health?.breaks);
	// how long until the first waiting clip airs, against how long the backlog takes
	const untilNeeded = $derived(q?.neededAt ? (Date.parse(q.neededAt) - now) / 1000 : null);
	const behind = $derived(
		q?.etaSeconds != null && untilNeeded != null && q.etaSeconds > untilNeeded
	);

	const segments = $derived<Segment[]>(
		b
			? [
					{
						key: 'ready',
						value: b.ready,
						color: renderColor.ready,
						label: m.admin_couchtv_render_ready()
					},
					{
						key: 'composed',
						value: b.composed,
						color: renderColor.composed,
						label: m.admin_couchtv_render_composed()
					},
					{
						key: 'failed',
						value: b.failed,
						color: renderColor.failed,
						label: m.admin_couchtv_render_failed()
					},
					{
						key: 'uncomposed',
						value: b.uncomposed,
						color: renderColor.uncomposed,
						label: m.admin_couchtv_render_uncomposed()
					}
				]
			: []
	);

	const disk = $derived(
		health
			? [
					{
						key: 'clips',
						label: m.admin_couchtv_health_clips({ count: health.cache.clips }),
						value: health.cache.clipBytes,
						display: formatBytes(health.cache.clipBytes)
					},
					{
						key: 'breaks',
						label: m.admin_couchtv_health_breaks({ count: health.cache.breaks }),
						value: health.cache.breakBytes,
						display: formatBytes(health.cache.breakBytes)
					}
				].sort((x, y) => y.value - x.value)
			: []
	);
</script>

<svelte:head>
	<title>{m.admin_couchtv_render_page_title()}</title>
</svelte:head>

{#if !health}
	{#if failed}
		<p class="text-sm text-muted">{m.admin_couchtv_health_failed()}</p>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
			{#each [0, 1, 2, 3] as i (i)}<Skeleton class="h-24 rounded-card" />{/each}
		</div>
	{/if}
{:else if q && b}
	<p class="mb-4 text-xs text-muted">
		{health.rendererSeenAt
			? m.admin_couchtv_health_seen({ when: formatRelative(health.rendererSeenAt, lang, now) })
			: m.admin_couchtv_health_never()}
	</p>

	<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
		{#each [{ key: 'queue', icon: Clapperboard, value: q.clips, label: m.admin_couchtv_health_queue(), sub: m.admin_couchtv_health_backlog( { minutes: Math.round(q.backlogSeconds / 6) / 10 } ) }, { key: 'speed', icon: Gauge, value: q.framesPerSecond ? q.framesPerSecond.toFixed(1) : '-', label: m.admin_couchtv_health_speed(), sub: m.admin_couchtv_health_assemblies( { count: q.assemblies } ) }, { key: 'eta', icon: Hourglass, value: q.etaSeconds != null ? formatEta(q.etaSeconds) : '-', label: m.admin_couchtv_health_eta(), sub: untilNeeded != null ? m.admin_couchtv_health_needed( { when: formatEta(Math.max(untilNeeded, 0)) } ) : m.admin_couchtv_health_nothing_waits() }, { key: 'risk', icon: AlertTriangle, value: b.atRisk, label: m.admin_couchtv_health_at_risk(), sub: m.admin_couchtv_health_upcoming( { count: b.upcoming } ) }] as tile, i (tile.key)}
			<div in:fly|global={{ y: 16, duration: 350, delay: Math.min(i * 55, 300) }}>
				<StatTile
					icon={tile.icon}
					value={tile.value}
					label={tile.label}
					sub={tile.sub}
					size="sm"
					live={tile.key === 'risk'}
				/>
			</div>
		{/each}
	</div>

	{#if behind}
		<p
			class="mt-4 rounded-card border border-danger/50 bg-danger/10 px-4 py-3 text-sm text-danger"
			role="status"
		>
			{m.admin_couchtv_health_behind()}
		</p>
	{/if}

	<div class="mt-6 grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
		<section class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_couchtv_health_breaks_ahead()}</h2>
			{#if b.upcoming === 0}
				<p class="text-xs text-faint">{m.admin_couchtv_health_no_breaks()}</p>
			{:else}
				<SegmentBar
					{segments}
					total={b.upcoming}
					class="h-2"
					title={(s) => `${s.label}: ${s.value}`}
				/>
				<ul class="mt-3 grid gap-1.5 text-xs sm:grid-cols-2">
					{#each segments as s (s.key)}
						<li class="flex items-center justify-between gap-3">
							<span class="flex items-center gap-2 text-muted">
								<span class="size-2 rounded-full" style="background: {s.color}"></span>{s.label}
							</span>
							<span class="text-faint tnum">{s.value}</span>
						</li>
					{/each}
				</ul>
				{#if b.nextUnready}
					<p class="mt-3 text-xs text-muted">
						{m.admin_couchtv_health_next_unready({
							when: formatRelative(b.nextUnready, lang, now)
						})}
					</p>
				{/if}
			{/if}
			<div class="mt-4 flex flex-wrap gap-2 text-xs">
				{#each [{ ok: b.evergreen, label: m.admin_couchtv_health_evergreen() }, { ok: b.standby, label: m.admin_couchtv_health_standby() }] as fallback (fallback.label)}
					<span
						class="inline-flex items-center gap-1.5 rounded-full border border-edge bg-surface-2 px-2.5 py-1"
					>
						<span class="size-1.5 rounded-full {fallback.ok ? 'bg-success' : 'bg-danger'}"></span>
						{fallback.label}: {fallback.ok
							? m.admin_couchtv_health_rendered()
							: m.admin_couchtv_health_missing()}
					</span>
				{/each}
			</div>
		</section>

		<section class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-3 flex items-center gap-2 text-sm font-semibold text-muted">
				<HardDrive class="size-4" />{m.admin_couchtv_health_disk()}
			</h2>
			<RankedList items={disk} />
		</section>
	</div>

	<section class="mt-6 rounded-card border border-edge bg-surface/40 p-6">
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_couchtv_health_failures()}</h2>
		{#if health.failures.length === 0}
			<p class="text-xs text-faint">{m.admin_couchtv_health_no_failures()}</p>
		{:else}
			<ul class="divide-y divide-edge/50 text-xs">
				{#each health.failures as f (f.id)}
					<li class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 py-2">
						<span class="font-medium">
							{f.kind === 'clip' ? m.admin_couchtv_health_clip() : m.admin_couchtv_break()} · {f.what}
						</span>
						<span class="text-faint">{formatRelative(f.at, lang, now)}</span>
						<span class="w-full break-words text-muted">{f.error}</span>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}
