<script lang="ts">
	import { ImageOff, Trash2, Undo2 } from 'lucide-svelte';
	import { untrack } from 'svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import * as m from '$lib/paraglide/messages';
	import { getStreamFramePath, type CouchTvTrailerCut } from '../api';
	import type { AiredFile } from '../files';
	import type { Grants } from '../media';
	import { formatLength } from '../time';
	import {
		drop,
		dropAll,
		footageSeconds,
		keep,
		keptScenes,
		sceneKey,
		stillSeconds,
		untouched,
		verdict,
		type TrailerEdit
	} from '../trailer';

	// The scenes a title's trailer is cut from, as the picker chose them: each shown by a
	// still from its start and one from its middle (the frame endpoint, through the file's media
	// grant) over what is said in it. The admin drops the ones that must not air, or all of them,
	// and sees at once whether what is left makes a trailer. The dropped stay on screen, dimmed,
	// until saved, so a slip is undone.
	let {
		scenes,
		edit,
		savedByAdmin,
		minScenes,
		analyzing,
		pickedBefore,
		files,
		grants,
		onedit
	}: {
		/** the scenes the server has, in the order they play */
		scenes: CouchTvTrailerCut[];
		edit: TrailerEdit;
		/** the saved trailer is the admin's pick */
		savedByAdmin: boolean;
		/** the fewest scenes that make a trailer */
		minScenes: number;
		analyzing: boolean;
		/** the picker has chosen at least once (its empty answer counts) */
		pickedBefore: boolean;
		files: AiredFile[];
		grants: Grants;
		onedit: (next: TrailerEdit) => void;
	} = $props();

	const kept = $derived(keptScenes(scenes, edit));
	const outcome = $derived(verdict(kept.length, minScenes));
	const handedBack = $derived(!edit.picked && savedByAdmin);

	let stills = $state<Record<string, { first: string; middle: string } | null>>({});

	$effect(() => {
		for (const scene of scenes) {
			const key = sceneKey(scene);
			if (untrack(() => key in stills)) continue;
			const file = files.find((f) => f.fileId === scene.mediaFileId);
			if (!file) {
				stills[key] = null;
				continue;
			}
			grants
				.get(file)
				.then((info) => {
					// the grant is for the file playback picks, which is the one the channel airs
					if (info.mediaFileId !== scene.mediaFileId) {
						stills[key] = null;
						return;
					}
					const at = stillSeconds(scene);
					stills[key] = {
						first: getStreamFramePath(info.grant, { t: at.first, exact: true }),
						middle: getStreamFramePath(info.grant, { t: at.middle, exact: true })
					};
				})
				.catch(() => (stills[key] = null));
		}
	});

	const fileLabel = (id: string) => files.find((f) => f.fileId === id)?.label ?? '';
</script>

<div class="space-y-3">
	<p class="text-[11px] text-faint">{m.admin_couchtv_trailer_hint({ min: minScenes })}</p>
	{#if analyzing && !savedByAdmin && (scenes.length > 0 || pickedBefore)}
		<p class="text-[11px] text-muted" role="status">{m.admin_couchtv_trailer_analyzing()}</p>
	{/if}

	{#if handedBack}
		<div
			class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-edge bg-surface-2/50 px-3 py-2.5"
		>
			<p class="text-xs text-muted" role="status">{m.admin_couchtv_trailer_repick_note()}</p>
			<Button
				type="button"
				variant="secondary"
				size="sm"
				onclick={() => onedit(untouched(savedByAdmin))}
			>
				{m.admin_couchtv_trailer_repick_undo()}
			</Button>
		</div>
	{:else}
		{#if scenes.length === 0}
			<p class="rounded-lg bg-surface px-3 py-4 text-center text-xs text-muted">
				{#if edit.picked}
					{m.admin_couchtv_trailer_none_chosen()}
				{:else if pickedBefore}
					{m.admin_couchtv_trailer_found_none({ min: minScenes })}
				{:else if analyzing}
					{m.admin_couchtv_trailer_picking()}
				{:else}
					{m.admin_couchtv_trailer_unpicked()}
				{/if}
			</p>
		{:else}
			<ol class="space-y-2" aria-label={m.admin_couchtv_trailer_scenes()}>
				{#each scenes as scene, i (sceneKey(scene))}
					{@const key = sceneKey(scene)}
					{@const still = stills[key]}
					{@const dropped = edit.dropped.includes(key)}
					<li
						class="flex flex-wrap items-start gap-x-4 gap-y-3 rounded-lg border border-edge bg-surface p-3"
					>
						<div class="flex shrink-0 gap-2 {dropped ? 'opacity-40' : ''}">
							{#each [still?.first, still?.middle] as src, k (k)}
								<div class="relative aspect-video w-36 overflow-hidden rounded bg-surface-2">
									{#if src}
										<img {src} alt="" class="size-full object-cover" loading="lazy" />
									{:else if still === null}
										<div class="flex size-full items-center justify-center text-faint">
											<ImageOff class="size-5" />
										</div>
									{/if}
									{#if k === 0}
										<span
											class="absolute top-1.5 left-1.5 rounded bg-black/70 px-1.5 py-0.5 text-[10px] font-semibold text-white tnum"
										>
											{i + 1}
										</span>
									{/if}
								</div>
							{/each}
						</div>
						<div class="min-w-0 flex-1 basis-56 space-y-1.5">
							<p class="flex flex-wrap items-center gap-x-2 text-[11px] text-faint tnum">
								{#if files.length > 1}
									<span class="truncate font-medium text-muted">{fileLabel(scene.mediaFileId)}</span
									>
								{/if}
								<span>
									{m.admin_couchtv_trailer_scene_at({
										at: formatLength(scene.startMs),
										length: (scene.durationMs / 1000).toFixed(1)
									})}
								</span>
							</p>
							{#if scene.lines.length > 0}
								<div
									class="space-y-1 border-l-2 border-edge pl-2.5 text-xs {dropped
										? 'text-muted'
										: 'text-text'}"
								>
									{#each scene.lines as line, k (k)}
										<p>{line}</p>
									{/each}
								</div>
							{:else}
								<p class="text-xs text-faint italic">{m.admin_couchtv_trailer_speech()}</p>
							{/if}
						</div>
						<div class="ml-auto flex shrink-0 items-center gap-2">
							{#if dropped}
								<Badge>{m.admin_couchtv_trailer_dropped()}</Badge>
								<button
									type="button"
									class="rounded-full p-1.5 text-muted hover:bg-surface-2 hover:text-text"
									aria-label={m.admin_couchtv_trailer_keep({ n: i + 1 })}
									onclick={() => onedit(keep(edit, key, savedByAdmin))}
								>
									<Undo2 class="size-4" />
								</button>
							{:else}
								<button
									type="button"
									class="rounded-full p-1.5 text-danger hover:bg-danger/15"
									aria-label={m.admin_couchtv_trailer_drop({ n: i + 1 })}
									onclick={() => onedit(drop(edit, key))}
								>
									<Trash2 class="size-4" />
								</button>
							{/if}
						</div>
					</li>
				{/each}
			</ol>

			<div class="space-y-1" aria-live="polite">
				{#if outcome === 'none'}
					<p class="text-xs text-muted">{m.admin_couchtv_trailer_none_chosen()}</p>
				{:else}
					<p class="text-xs text-muted tnum">
						{m.admin_couchtv_trailer_footage({
							count: kept.length,
							length: footageSeconds(kept)
						})}
					</p>
					{#if outcome === 'too_few'}
						<p class="text-xs text-text">
							{m.admin_couchtv_trailer_too_few({ min: minScenes })}
						</p>
					{/if}
				{/if}
			</div>
		{/if}

		<div>
			<Button
				type="button"
				variant="secondary"
				size="sm"
				disabled={edit.picked && kept.length === 0}
				onclick={() => onedit(dropAll(scenes))}
			>
				<Trash2 class="size-4" />{m.admin_couchtv_trailer_none()}
			</Button>
		</div>
	{/if}
</div>
