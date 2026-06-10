<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import { AudioLines, EllipsisVertical, ListPlus, Play, Plus } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as musicApi from '$lib/features/music/api';
	import type { TrackItem } from '$lib/features/music/api';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';
	import { formatClock } from '$lib/utils/format';

	let { tracks, showAlbum = false }: { tracks: TrackItem[]; showAlbum?: boolean } = $props();

	const isPlaying = (track: TrackItem) => player.current?.id === track.id;

	async function addToPlaylist(track: TrackItem) {
		try {
			const playlists = await musicApi.listPlaylists();
			if (playlists.length === 0) {
				const created = await musicApi.createPlaylist('My Playlist');
				await musicApi.addPlaylistTrack(created.id, track.id);
				toast.success(`Added to “${created.name}”`);
				return;
			}
			// add to the most recently updated playlist (quick action)
			await musicApi.addPlaylistTrack(playlists[0].id, track.id);
			toast.success(`Added to “${playlists[0].name}”`);
		} catch {
			toast.error('Failed to add to playlist');
		}
	}
</script>

<ul class="overflow-hidden rounded-card border border-edge bg-surface/40">
	{#each tracks as track, i (track.id)}
		<li
			class="group flex items-center gap-4 border-b border-edge/40 px-4 py-2.5 transition-colors
				last:border-0 hover:bg-surface-2/50 {track.mediaFileId === null ? 'opacity-50' : ''}"
		>
			<button
				class="flex w-7 items-center justify-center"
				onclick={() => player.playQueue(tracks, i)}
				disabled={track.mediaFileId === null}
				aria-label="Play {track.name}"
			>
				{#if isPlaying(track)}
					<AudioLines class="size-4 animate-pulse text-accent" />
				{:else}
					<span class="text-xs text-faint tnum group-hover:hidden">{track.trackNumber}</span>
					<Play class="hidden size-3.5 fill-current text-text group-hover:block" />
				{/if}
			</button>

			<div class="min-w-0 flex-1">
				<p class="truncate text-sm font-medium {isPlaying(track) ? 'text-accent' : ''}">
					{track.name}
				</p>
				<p class="truncate text-xs text-faint">
					{track.trackArtist ?? track.artistName}{showAlbum ? ` · ${track.albumName}` : ''}
				</p>
			</div>

			<span class="text-xs text-faint tnum">{formatClock(track.durationSeconds)}</span>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					class="rounded-full p-1.5 text-faint opacity-0 transition-opacity group-hover:opacity-100
						hover:bg-surface hover:text-text data-[state=open]:opacity-100"
					aria-label="Track actions"
				>
					<EllipsisVertical class="size-4" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Portal>
					<DropdownMenu.Content
						align="end"
						sideOffset={4}
						class="z-50 w-44 animate-pop-in rounded-card border border-edge bg-surface-2 p-1 shadow-xl"
					>
						<DropdownMenu.Item
							class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-muted
								outline-none data-highlighted:bg-surface data-highlighted:text-text"
							onSelect={() => player.addToQueue(track)}
						>
							<Plus class="size-3.5" />
							Add to queue
						</DropdownMenu.Item>
						<DropdownMenu.Item
							class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-muted
								outline-none data-highlighted:bg-surface data-highlighted:text-text"
							onSelect={() => addToPlaylist(track)}
						>
							<ListPlus class="size-3.5" />
							Add to playlist
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		</li>
	{/each}
</ul>
