<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { ImagePlus, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl, artworkVer } from '$lib/features/catalog/api';
	import * as libraryApi from '$lib/features/library/api';
	import type { Artwork } from '$lib/features/library/api';
	import Flag from '$lib/components/ui/Flag.svelte';
	import { langLabel } from '$lib/i18n/content-langs';
	import * as m from '$lib/paraglide/messages';

	let {
		titleId,
		languages,
		artwork
	}: {
		titleId: string;
		languages: string[];
		artwork: Artwork[];
	} = $props();

	// one slot per content language ('' is a logo not tied to a language, which
	// only a title without content languages offers), plus any stored logo whose
	// language is not among them
	const slots = $derived.by(() => {
		const logos = artwork.filter((a) => a.kind === 'logo');
		const langs = languages.length ? [...languages] : [''];
		for (const logo of logos) {
			const lang = logo.lang ?? '';
			if (!langs.includes(lang)) langs.push(lang);
		}
		return langs.map((lang) => ({ lang, logo: logos.find((a) => (a.lang ?? '') === lang) }));
	});

	let fileInput = $state<HTMLInputElement>();
	let target = '';

	function pick(lang: string) {
		target = lang;
		fileInput?.click();
	}

	async function upload(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		const form = new FormData();
		form.set('ownerKind', 'title');
		form.set('ownerId', titleId);
		form.set('kind', 'logo');
		if (target) form.set('lang', target);
		form.set('file', file);
		try {
			await libraryApi.adminUploadArtwork(form);
			toast.success(m.library_logo_updated());
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_upload_failed());
		}
	}

	async function remove(logo: Artwork) {
		try {
			await libraryApi.adminDeleteArtwork(logo.id);
			invalidateAll();
		} catch {
			toast.error(m.library_delete_artwork_failed());
		}
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="text-sm font-semibold text-muted">{m.library_logos()}</h2>
	<p class="mt-1 mb-4 text-xs text-faint">{m.library_logos_hint()}</p>

	<ul class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		{#each slots as slot (slot.lang)}
			<li class="overflow-hidden rounded-card border border-edge/60">
				<div class="stage flex h-28 items-center justify-center p-4">
					{#if slot.logo}
						<img
							src={artworkUrl(slot.logo.id, artworkVer(slot.logo.createdAt), 'w780')}
							alt=""
							class="max-h-full max-w-full object-contain"
						/>
					{:else}
						<span class="text-xs text-faint">{m.library_no_logo()}</span>
					{/if}
				</div>
				<div class="flex items-center gap-2 border-t border-edge/60 px-3 py-2 text-xs">
					{#if slot.lang}<Flag code={slot.lang} />{/if}
					<span class="min-w-0 flex-1 truncate font-medium">
						{slot.lang ? langLabel(slot.lang) : m.library_logo_any_language()}
					</span>
					<button
						type="button"
						class="rounded-full p-1.5 text-muted transition-colors hover:text-text"
						onclick={() => pick(slot.lang)}
						title={slot.logo ? m.library_replace_logo() : m.library_upload_logo()}
						aria-label={slot.logo ? m.library_replace_logo() : m.library_upload_logo()}
					>
						<ImagePlus class="size-3.5" />
					</button>
					{#if slot.logo}
						{@const logo = slot.logo}
						<button
							type="button"
							class="rounded-full p-1.5 text-muted transition-colors hover:text-danger"
							onclick={() => remove(logo)}
							title={m.library_remove_logo()}
							aria-label={m.library_remove_logo()}
						>
							<Trash2 class="size-3.5" />
						</button>
					{/if}
				</div>
			</li>
		{/each}
	</ul>
</div>

<input
	bind:this={fileInput}
	type="file"
	accept=".png"
	class="hidden"
	onchange={(e) => {
		upload(e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>

<style lang="scss">
	// a dark checkerboard shows the transparency of the (usually light) logos
	.stage {
		--square: var(--color-surface-2);
		background-color: var(--color-bg);
		background-image:
			linear-gradient(
				45deg,
				var(--square) 25%,
				transparent 25%,
				transparent 75%,
				var(--square) 75%
			),
			linear-gradient(45deg, var(--square) 25%, transparent 25%, transparent 75%, var(--square) 75%);
		background-position:
			0 0,
			8px 8px;
		background-size: 16px 16px;
	}
</style>
