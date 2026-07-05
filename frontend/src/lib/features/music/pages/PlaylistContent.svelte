<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ArrowDown, ArrowUp, ListMusic, Pencil, Play, Trash2, X } from 'lucide-svelte';
	import { flip } from 'svelte/animate';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import * as musicApi from '$lib/features/music/api';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import { formatClock } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: Awaited<ReturnType<typeof musicApi.getPlaylist>> } = $props();

	// local copy so reorder/remove can be optimistic; rewritten on navigation
	let entries = $derived(data.entries);

	let renameOpen = $state(false);
	let newName = $state(data.playlist.name);
	let confirmDelete = $state(false);
	const renameForm = new FormState(() => ({ newName }));

	function openRename() {
		newName = data.playlist.name;
		renameForm.reset();
		renameOpen = true;
	}

	const tracks = $derived(entries.map((e) => e.track));

	function playAll() {
		player.playQueue(tracks);
	}

	async function move(index: number, dir: -1 | 1) {
		const target = index + dir;
		if (target < 0 || target >= entries.length) return;
		const next = [...entries];
		[next[index], next[target]] = [next[target], next[index]];
		entries = next;
		try {
			await musicApi.reorderPlaylist(
				data.playlist.id,
				next.map((e) => e.entryId)
			);
		} catch {
			toast.error(m.music_reorder_failed());
			invalidateAll();
		}
	}

	async function removeEntry(entryId: string) {
		entries = entries.filter((e) => e.entryId !== entryId);
		try {
			await musicApi.removePlaylistEntry(data.playlist.id, entryId);
		} catch {
			toast.error(m.music_remove_track_failed());
			invalidateAll();
		}
	}

	async function rename(e: SubmitEvent) {
		e.preventDefault();
		try {
			await musicApi.renamePlaylist(data.playlist.id, newName);
			renameOpen = false;
			invalidateAll();
		} catch {
			toast.error(m.music_rename_failed());
		}
	}

	async function deletePlaylist() {
		try {
			await musicApi.deletePlaylist(data.playlist.id);
			goto('/music');
		} catch {
			toast.error(m.music_delete_failed());
		}
	}

	const isPlaying = (trackId: string) => player.current?.id === trackId;
</script>

<svelte:head>
	<title>{m.music_playlist_title({ name: data.playlist.name })}</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-6 pt-28 pb-16">
	<div class="mb-8 flex items-end justify-between gap-4">
		<div class="min-w-0">
			<p class="eyebrow mb-2">{m.music_playlist()}</p>
			<h1 class="truncate text-3xl font-extrabold tracking-tight sm:text-4xl">
				{data.playlist.name}
			</h1>
			<p class="mt-2 text-sm text-faint tnum">{m.music_track_count({ count: entries.length })}</p>
		</div>
		<div class="flex shrink-0 gap-2">
			<Button onclick={playAll} disabled={entries.length === 0}>
				<Play class="size-4 fill-current" />
				{m.common_play()}
			</Button>
			<Button variant="ghost" size="md" onclick={openRename} aria-label={m.music_rename()}>
				<Pencil class="size-4" />
			</Button>
			<Button
				variant="ghost"
				size="md"
				onclick={() => (confirmDelete = true)}
				aria-label={m.music_delete_playlist()}
			>
				<Trash2 class="size-4" />
			</Button>
		</div>
	</div>

	{#if entries.length === 0}
		<EmptyState title={m.music_empty_playlist_title()} message={m.music_empty_playlist_message()}>
			<ListMusic class="size-5 text-faint" />
		</EmptyState>
	{:else}
		<ul class="overflow-hidden rounded-card border border-edge bg-surface/40">
			{#each entries as entry, i (entry.entryId)}
				<li
					animate:flip={{ duration: 250 }}
					class="group flex items-center gap-3 border-b border-edge/40 px-4 py-2.5 transition-colors
						last:border-0 hover:bg-surface-2/50"
				>
					<button
						class="w-6 text-left text-xs text-faint tnum"
						onclick={() => player.playQueue(tracks, i)}
						aria-label={m.music_play_from_here()}
					>
						{i + 1}
					</button>
					<div class="min-w-0 flex-1">
						<p
							class="truncate text-sm font-medium {isPlaying(entry.track.id) ? 'text-accent' : ''}"
						>
							{entry.track.name}
						</p>
						<p class="truncate text-xs text-faint">
							{entry.track.trackArtist ?? entry.track.artistName} · {entry.track.albumName}
						</p>
					</div>
					<span class="text-xs text-faint tnum">{formatClock(entry.track.durationSeconds)}</span>
					<span class="flex gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
						<button
							class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
							disabled={i === 0}
							onclick={() => move(i, -1)}
							aria-label={m.music_move_up()}
						>
							<ArrowUp class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
							disabled={i === entries.length - 1}
							onclick={() => move(i, 1)}
							aria-label={m.music_move_down()}
						>
							<ArrowDown class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint hover:text-danger"
							onclick={() => removeEntry(entry.entryId)}
							aria-label={m.common_remove()}
						>
							<X class="size-3.5" />
						</button>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<Modal bind:open={renameOpen} title={m.music_rename_playlist()}>
	<form onsubmit={rename} class="space-y-4">
		<Input label={m.music_name_label()} bind:value={newName} required />
		<div class="flex justify-end">
			<Button type="submit" disabled={!renameForm.dirty}>{m.common_save()}</Button>
		</div>
	</form>
</Modal>

<Confirm
	bind:open={confirmDelete}
	title={m.music_delete_playlist_confirm_title({ name: data.playlist.name })}
	message={m.music_delete_playlist_confirm_message()}
	onconfirm={deletePlaylist}
/>
