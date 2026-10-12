<script lang="ts">
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { problemMessage } from '$lib/api/problem';
	import * as m from '$lib/paraglide/messages';
	import {
		adminGetMediaMarkers,
		adminUpdateMediaMarkers,
		type MediaMarker,
		type MediaMarkerEdit,
		type MediaMarkerKind,
		type MediaMarkers
	} from '../api';
	import { markerLabel } from '../labels';
	import type { AiredFile } from '../files';
	import { formatLength, parseLength } from '../time';

	// Where a file's intro and credits are, as the analysis found them, and the admin's
	// word over it: other times, "none", or back to the analysis.
	let { files, movie }: { files: AiredFile[]; movie: boolean } = $props();

	// svelte-ignore state_referenced_locally
	let fileId = $state(files[0]?.fileId ?? '');
	let data = $state<MediaMarkers | null>(null);
	let failed = $state(false);
	let busy = $state(false);
	let edits = $state<Record<string, { start: string; end: string; base: string }>>({});

	const kinds = $derived<MediaMarkerKind[]>(
		movie ? ['credits', 'credits_music'] : ['intro', 'credits']
	);
	const fileItems = $derived(files.map((f) => ({ value: f.fileId, label: f.label })));

	const of = (kind: MediaMarkerKind, source: MediaMarker['source']) =>
		data?.markers.find((mk) => mk.kind === kind && mk.source === source);

	// a movie's credits run to its end, so they have a start only
	const ranged = (kind: MediaMarkerKind) => !(movie && kind === 'credits');

	const span = (mk: MediaMarker | undefined) =>
		!mk
			? ''
			: mk.none
				? m.admin_couchtv_marker_none()
				: mk.endMs != null
					? `${formatLength(mk.startMs ?? 0)}-${formatLength(mk.endMs)}`
					: m.admin_couchtv_marker_from({ at: formatLength(mk.startMs ?? 0) });

	function adopt(next: MediaMarkers) {
		data = next;
		const out: typeof edits = {};
		for (const kind of kinds) {
			const shown = of(kind, 'admin') ?? of(kind, 'auto');
			const start =
				shown && !shown.none && shown.startMs != null ? formatLength(shown.startMs) : '';
			const end = shown && !shown.none && shown.endMs != null ? formatLength(shown.endMs) : '';
			out[kind] = { start, end, base: `${start}|${end}` };
		}
		edits = out;
	}

	$effect(() => {
		const id = fileId;
		if (!id) return;
		let live = true;
		data = null;
		failed = false;
		adminGetMediaMarkers(id)
			.then((d) => {
				if (live) adopt(d);
			})
			.catch(() => {
				if (live) failed = true;
			});
		return () => {
			live = false;
		};
	});

	// the admin's rows of every other kind stay as they are
	function others(kind: MediaMarkerKind): MediaMarkerEdit[] {
		return (data?.markers ?? [])
			.filter((mk) => mk.source === 'admin' && mk.kind !== kind)
			.map((mk) =>
				mk.none
					? { kind: mk.kind, none: true }
					: { kind: mk.kind, startMs: mk.startMs, endMs: mk.endMs }
			);
	}

	async function put(kind: MediaMarkerKind, edit: MediaMarkerEdit | null, done: string) {
		busy = true;
		try {
			const markers = edit ? [...others(kind), edit] : others(kind);
			adopt(await adminUpdateMediaMarkers(fileId, { markers }));
			toast.success(done);
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_markers_failed()));
		} finally {
			busy = false;
		}
	}

	function setTimes(kind: MediaMarkerKind) {
		const e = edits[kind];
		const startMs = parseLength(e.start);
		const endMs = e.end ? parseLength(e.end) : undefined;
		if (Number.isNaN(startMs) || (endMs !== undefined && Number.isNaN(endMs))) {
			toast.error(m.admin_couchtv_marker_bad_time());
			return;
		}
		put(kind, { kind, startMs, endMs }, m.admin_couchtv_markers_saved());
	}
</script>

<div class="space-y-3">
	{#if fileItems.length > 1}
		<Select
			items={fileItems}
			bind:value={fileId}
			label={m.admin_couchtv_file()}
			class="max-w-full justify-between"
		/>
	{/if}

	{#if failed}
		<p class="text-xs text-muted">{m.admin_couchtv_markers_load_failed()}</p>
	{:else if !data}
		<div class="h-24 animate-pulse rounded-lg bg-surface"></div>
	{:else}
		{#if data.analyzedVersion === 0}
			<p class="text-xs text-faint">{m.admin_couchtv_markers_not_analyzed()}</p>
		{/if}
		<ul class="space-y-3">
			{#each kinds as kind (kind)}
				{@const auto = of(kind, 'auto')}
				{@const admin = of(kind, 'admin')}
				{@const e = edits[kind]}
				<li class="rounded-lg border border-edge bg-surface p-3" aria-label={markerLabel(kind)}>
					<div class="flex flex-wrap items-baseline justify-between gap-2">
						<span class="text-sm font-medium">{markerLabel(kind)}</span>
						<span class="text-xs text-muted">
							{m.admin_couchtv_marker_auto()}:
							{auto ? span(auto) : data.analyzedVersion ? m.admin_couchtv_marker_not_found() : '-'}
							{#if admin}
								· <span class="text-accent-ink"
									>{m.admin_couchtv_marker_admin()}: {span(admin)}</span
								>
							{/if}
						</span>
					</div>
					{#if e}
						<div class="mt-2 flex flex-wrap items-end gap-2">
							<Input
								label={m.admin_couchtv_marker_start()}
								bind:value={e.start}
								class="w-28"
								placeholder="0:00"
							/>
							{#if ranged(kind)}
								<Input
									label={m.admin_couchtv_marker_end()}
									bind:value={e.end}
									class="w-28"
									placeholder="0:00"
								/>
							{/if}
							<Button
								size="sm"
								disabled={busy ||
									!e.start ||
									(ranged(kind) && !e.end) ||
									`${e.start}|${e.end}` === e.base}
								onclick={() => setTimes(kind)}
							>
								{m.common_save()}
							</Button>
							<Button
								size="sm"
								variant="secondary"
								disabled={busy || admin?.none}
								onclick={() => put(kind, { kind, none: true }, m.admin_couchtv_markers_saved())}
							>
								{m.admin_couchtv_marker_set_none()}
							</Button>
							{#if admin}
								<Button
									size="sm"
									variant="ghost"
									disabled={busy}
									onclick={() => put(kind, null, m.admin_couchtv_markers_reset())}
								>
									{m.admin_couchtv_back_to_analysis()}
								</Button>
							{/if}
						</div>
					{/if}
				</li>
			{/each}
		</ul>
		<p class="text-[11px] text-faint">{m.admin_couchtv_markers_hint()}</p>
	{/if}
</div>
