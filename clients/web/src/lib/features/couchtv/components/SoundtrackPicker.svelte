<script lang="ts">
	import { Play, Search, Square } from 'lucide-svelte';
	import { onDestroy } from 'svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { formatRelative } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
	import {
		adminCouchTvSoundtrackPreviewPath,
		type CouchTvSoundtrack,
		type CouchTvTitle
	} from '../api';
	import { catalogLabel } from '../labels';

	// The soundtracks the finder found on the web for a title's promos: each heard from the
	// server's copy of its 30-second preview (never the catalog's own servers), one chosen,
	// and a search again, the finder's own or with the admin's words.
	let {
		title,
		chosen,
		onchoose,
		onsearch
	}: {
		title: CouchTvTitle;
		/** the admin's choice, unsaved or saved; null leaves it to the finder */
		chosen: string | null;
		onchoose: (id: string) => void;
		onsearch: (query: string) => Promise<void>;
	} = $props();

	const lang = $derived(currentLang());
	const percent = $derived(
		new Intl.NumberFormat(lang, { style: 'percent', maximumFractionDigits: 0 })
	);
	// svelte-ignore state_referenced_locally
	let query = $state(title.soundtrackQuery ?? '');
	let searching = $state(false);

	let audio = $state<HTMLAudioElement>();
	let playing = $state<string | null>(null);
	let playError = $state('');

	// what the promos play now, as the server has it
	const inPromos = $derived(
		title.promoBed === 'soundtrack' && title.soundtrack?.ready
			? title.soundtrack.soundtrackId
			: null
	);

	const searched = $derived.by(() => {
		if (title.searchingSoundtrack) return m.admin_couchtv_soundtrack_searching();
		if (!title.soundtrackSearchedAt) return m.admin_couchtv_soundtrack_not_searched();
		const when = formatRelative(title.soundtrackSearchedAt, lang);
		return title.soundtrackQuery
			? m.admin_couchtv_soundtrack_searched_for({ when, query: title.soundtrackQuery })
			: m.admin_couchtv_soundtrack_searched({ when });
	});

	async function toggle(s: CouchTvSoundtrack) {
		if (!audio) return;
		if (playing === s.id) return stop();
		playError = '';
		audio.src = adminCouchTvSoundtrackPreviewPath(s.id);
		try {
			await audio.play();
			playing = s.id;
		} catch {
			playing = null;
			playError = m.admin_couchtv_soundtrack_unplayable();
		}
	}

	function stop() {
		audio?.pause();
		playing = null;
	}

	async function search(e: SubmitEvent) {
		e.preventDefault();
		searching = true;
		try {
			await onsearch(query.trim());
		} finally {
			searching = false;
		}
	}

	onDestroy(stop);
</script>

<div class="space-y-3">
	<div class="flex flex-wrap items-baseline justify-between gap-2">
		<h4 class="text-xs font-medium text-muted">{m.admin_couchtv_soundtrack()}</h4>
		<span class="text-[11px] text-faint" aria-live="polite">{searched}</span>
	</div>
	<p class="text-[11px] text-faint">{m.admin_couchtv_soundtrack_hint()}</p>

	{#if title.soundtracks.length === 0}
		<p class="rounded-lg bg-surface px-3 py-4 text-center text-xs text-muted">
			{m.admin_couchtv_soundtrack_none()}
		</p>
	{:else}
		<ul class="space-y-1" aria-label={m.admin_couchtv_soundtrack_list()}>
			{#each title.soundtracks as s (s.id)}
				<li
					class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg px-2 py-1.5
						{chosen === s.id ? 'bg-accent-soft' : inPromos === s.id ? 'bg-surface' : ''}"
				>
					<Button
						variant="ghost"
						size="sm"
						aria-label={playing === s.id
							? m.admin_couchtv_soundtrack_stop({ track: s.track })
							: m.admin_couchtv_soundtrack_play({ track: s.track })}
						onclick={() => toggle(s)}
					>
						{#if playing === s.id}<Square class="size-4" />{:else}<Play class="size-4" />{/if}
					</Button>
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm font-medium">{s.track}</p>
						<p class="truncate text-xs text-muted">
							{s.artist} · {s.album}{s.year ? ` (${s.year})` : ''}
						</p>
					</div>
					<span class="text-[11px] text-faint tnum">
						{catalogLabel(s.provider)} · {m.admin_couchtv_soundtrack_confidence({
							percent: percent.format(s.confidence)
						})}
					</span>
					{#if inPromos === s.id}<Badge>{m.admin_couchtv_soundtrack_in_promos()}</Badge>{/if}
					<Button
						variant={chosen === s.id ? 'primary' : 'secondary'}
						size="sm"
						aria-pressed={chosen === s.id}
						aria-label={m.admin_couchtv_soundtrack_use_named({ track: s.track })}
						onclick={() => onchoose(s.id)}
					>
						{m.admin_couchtv_soundtrack_use()}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
	{#if playError}<p class="text-xs text-danger">{playError}</p>{/if}

	<form class="flex flex-wrap items-end gap-3" onsubmit={search}>
		<Input
			label={m.admin_couchtv_soundtrack_query()}
			placeholder={m.admin_couchtv_soundtrack_query_placeholder()}
			bind:value={query}
			maxlength={200}
			class="min-w-48 flex-1"
		/>
		<Button
			type="submit"
			variant="secondary"
			size="sm"
			loading={searching}
			disabled={title.searchingSoundtrack}
		>
			<Search class="size-4" />{m.admin_couchtv_soundtrack_search()}
		</Button>
	</form>
	<audio bind:this={audio} onended={stop} class="hidden"></audio>
</div>
