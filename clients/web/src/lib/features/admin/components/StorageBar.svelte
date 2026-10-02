<script lang="ts">
	import type { StorageInfo } from '$lib/features/jobs/api';
	import SegmentBar, { type Segment } from '$lib/components/ui/SegmentBar.svelte';
	import { categoryStyle } from './storageColors';
	import * as m from '$lib/paraglide/messages';

	// Segments are sized against `budget` (Couchverse usage + free), so the
	// leftover track shows the free space available to Couchverse.
	let { storage, class: cls = 'h-1.5' }: { storage: StorageInfo; class?: string } = $props();

	const segments = $derived<Segment[]>(
		storage.categories.map((cat) => {
			const style = categoryStyle[cat.kind] ?? { label: cat.kind, color: '#5b6072' };
			return { key: cat.kind, value: cat.bytes, color: style.color, label: style.label };
		})
	);
</script>

<SegmentBar
	{segments}
	total={storage.budget}
	class={cls}
	title={(s) => m.admin_storage_bar_title({ label: s.label, bytes: s.value })}
/>
