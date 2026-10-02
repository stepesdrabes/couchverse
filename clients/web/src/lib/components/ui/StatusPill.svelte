<script lang="ts">
	import type { ContentStatus } from '$lib/features/catalog/types';
	import * as m from '$lib/paraglide/messages';

	let { status }: { status: ContentStatus } = $props();

	const styles: Record<ContentStatus, { dot: string; text: string; label: () => string }> = {
		published: { dot: 'bg-success', text: 'text-success', label: m.common_published },
		processing: { dot: 'bg-accent animate-pulse', text: 'text-accent', label: m.common_processing },
		draft: { dot: 'bg-faint', text: 'text-muted', label: m.common_draft },
		hidden: { dot: 'bg-faint', text: 'text-faint', label: m.common_hidden }
	};
	const style = $derived(styles[status]);
</script>

<span
	class="inline-flex items-center gap-1.5 rounded-full border border-edge bg-surface-2 py-0.5 pr-2.5 pl-2 text-xs font-medium {style.text}"
>
	<span class="size-1.5 rounded-full {style.dot}"></span>
	{style.label()}
</span>
