<script lang="ts">
	import { ArrowLeft, ArrowRight, ImageOff, Trash2 } from 'lucide-svelte';
	import { untrack } from 'svelte';
	import * as m from '$lib/paraglide/messages';
	import { getStreamFramePath, type CouchTvShot } from '../api';
	import type { AiredFile } from '../files';
	import type { Grants } from '../media';
	import { formatLength } from '../time';

	// The footage a title's promos cut between, in airing order, each shown by a still
	// from its middle (the frame endpoint, through the file's media grant).
	let {
		shots,
		files,
		grants,
		onchange
	}: {
		shots: CouchTvShot[];
		files: AiredFile[];
		grants: Grants;
		onchange: (shots: CouchTvShot[]) => void;
	} = $props();

	const keyOf = (s: CouchTvShot) => `${s.mediaFileId}:${s.startMs}`;
	let stills = $state<Record<string, string | null>>({});

	$effect(() => {
		for (const shot of shots) {
			const key = keyOf(shot);
			const file = files.find((f) => f.fileId === shot.mediaFileId);
			if (untrack(() => key in stills)) continue;
			if (!file) {
				stills[key] = null;
				continue;
			}
			grants
				.get(file)
				.then((info) => {
					// the grant is for the file playback picks, which is the one the channel airs
					stills[key] =
						info.mediaFileId === shot.mediaFileId
							? getStreamFramePath(info.grant, {
									t: Math.floor((shot.startMs + shot.durationMs / 2) / 1000)
								})
							: null;
				})
				.catch(() => (stills[key] = null));
		}
	});

	function move(i: number, by: -1 | 1) {
		const next = [...shots];
		[next[i], next[i + by]] = [next[i + by], next[i]];
		onchange(next);
	}

	const fileLabel = (id: string) => files.find((f) => f.fileId === id)?.label ?? '';
</script>

{#if shots.length === 0}
	<p class="text-xs text-faint">{m.admin_couchtv_shots_none()}</p>
{:else}
	<ol class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		{#each shots as shot, i (keyOf(shot))}
			{@const still = stills[keyOf(shot)]}
			<li class="overflow-hidden rounded-lg border border-edge bg-surface">
				<div class="relative aspect-video bg-surface-2">
					{#if still}
						<img src={still} alt="" class="size-full object-cover" loading="lazy" />
					{:else if still === null}
						<div class="flex size-full items-center justify-center text-faint">
							<ImageOff class="size-5" />
						</div>
					{/if}
					<span
						class="absolute top-1.5 left-1.5 rounded bg-black/70 px-1.5 py-0.5 text-[10px] font-semibold text-white tnum"
					>
						{i + 1}
					</span>
				</div>
				<div class="flex items-center justify-between gap-2 px-2.5 py-2">
					<div class="min-w-0 text-[11px]">
						{#if files.length > 1}<p class="truncate font-medium">
								{fileLabel(shot.mediaFileId)}
							</p>{/if}
						<p class="text-faint tnum">
							{m.admin_couchtv_shot_at({
								at: formatLength(shot.startMs),
								length: (shot.durationMs / 1000).toFixed(1)
							})}
						</p>
					</div>
					<div class="flex shrink-0 items-center">
						<button
							type="button"
							class="rounded-full p-1.5 text-muted hover:bg-surface-2 hover:text-text disabled:opacity-30"
							aria-label={m.admin_couchtv_shot_earlier({ n: i + 1 })}
							disabled={i === 0}
							onclick={() => move(i, -1)}
						>
							<ArrowLeft class="size-3.5" />
						</button>
						<button
							type="button"
							class="rounded-full p-1.5 text-muted hover:bg-surface-2 hover:text-text disabled:opacity-30"
							aria-label={m.admin_couchtv_shot_later({ n: i + 1 })}
							disabled={i === shots.length - 1}
							onclick={() => move(i, 1)}
						>
							<ArrowRight class="size-3.5" />
						</button>
						<button
							type="button"
							class="rounded-full p-1.5 text-danger hover:bg-danger/15"
							aria-label={m.admin_couchtv_shot_remove({ n: i + 1 })}
							onclick={() => onchange(shots.filter((_, k) => k !== i))}
						>
							<Trash2 class="size-3.5" />
						</button>
					</div>
				</div>
			</li>
		{/each}
	</ol>
{/if}
