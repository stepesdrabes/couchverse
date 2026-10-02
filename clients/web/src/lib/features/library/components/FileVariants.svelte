<script lang="ts">
	import { Clapperboard, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { MediaFile } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { TranscodeVariant } from '$lib/features/library/api';
	import { formatBytes } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	let { file }: { file: MediaFile } = $props();

	let variants = $state<TranscodeVariant[]>([]);

	async function refresh() {
		variants = await libraryApi.listVariants(file.id);
	}

	$effect(() => {
		refresh();
		// keep polling while anything is in flight
		const t = setInterval(() => {
			if (variants.some((v) => v.status === 'queued' || v.status === 'processing')) refresh();
		}, 3000);
		return () => clearInterval(t);
	});

	async function prepare() {
		try {
			const res = await libraryApi.enqueueTranscode(file.id);
			toast.success(m.library_queued({ items: res.queued.join(', ') }));
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_queue_transcode_failed());
		}
	}

	async function remove(variant: TranscodeVariant) {
		try {
			await libraryApi.deleteVariant(variant.id);
			refresh();
		} catch {
			toast.error(m.library_delete_variant_failed());
		}
	}

	const statusColor: Record<TranscodeVariant['status'], string> = {
		queued: 'text-muted',
		processing: 'text-accent',
		ready: 'text-success',
		failed: 'text-danger'
	};
</script>

{#if !file.directPlay && file.videoCodec}
	<div class="mt-1.5">
		{#if variants.length > 0}
			<ul class="space-y-0.5">
				{#each variants as variant (variant.id)}
					<li class="flex items-center gap-2 text-[11px]">
						<span class="font-semibold">{variant.name}</span>
						<span class={statusColor[variant.status]}>{variant.status}</span>
						<span class="text-faint">({variant.mode})</span>
						{#if variant.sizeBytes > 0}
							<span class="text-faint tnum">{formatBytes(variant.sizeBytes)}</span>
						{/if}
						<button
							class="rounded-full p-0.5 text-faint transition-colors hover:text-danger"
							onclick={() => remove(variant)}
							title={m.library_delete_variant()}
						>
							<Trash2 class="size-3" />
						</button>
					</li>
				{/each}
			</ul>
		{/if}
		<button
			class="mt-1 inline-flex items-center gap-1 text-[11px] font-medium text-accent hover:underline"
			onclick={prepare}
		>
			<Clapperboard class="size-3" />
			{m.library_prepare_hls()}
		</button>
	</div>
{/if}
