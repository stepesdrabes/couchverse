<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { X } from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	let {
		open = $bindable(false),
		title,
		description = '',
		children,
		footer
	}: {
		open?: boolean;
		title: string;
		description?: string;
		children: Snippet;
		footer?: Snippet;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay class="fixed inset-0 z-50 animate-fade-in bg-black/60 backdrop-blur-sm" />
		<Dialog.Content
			class="fixed top-1/2 left-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2
				animate-pop-in rounded-card border border-edge bg-surface-2 p-6 shadow-2xl shadow-black/50"
		>
			<div class="mb-4 flex items-start justify-between gap-4">
				<div>
					<Dialog.Title class="text-base font-semibold">{title}</Dialog.Title>
					{#if description}
						<Dialog.Description class="mt-1 text-sm text-muted">
							{description}
						</Dialog.Description>
					{/if}
				</div>
				<Dialog.Close
					class="rounded-full p-1.5 text-muted transition-colors hover:bg-surface hover:text-text"
				>
					<X class="size-4" />
				</Dialog.Close>
			</div>

			{@render children()}

			{#if footer}
				<div class="mt-6 flex justify-end gap-2">
					{@render footer()}
				</div>
			{/if}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
