<script lang="ts">
	import { AlertDialog } from 'bits-ui';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		title,
		message,
		confirmLabel = m.common_delete(),
		onconfirm
	}: {
		open?: boolean;
		title: string;
		message: string;
		confirmLabel?: string;
		onconfirm: () => void;
	} = $props();
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Portal>
		<AlertDialog.Overlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm" />
		<AlertDialog.Content
			class="fixed top-1/2 left-1/2 z-50 w-full max-w-sm -translate-x-1/2 -translate-y-1/2
				animate-pop-in rounded-card border border-edge bg-surface-2 p-6 shadow-2xl shadow-black/50"
		>
			<AlertDialog.Title class="text-base font-semibold">{title}</AlertDialog.Title>
			<AlertDialog.Description class="mt-2 text-sm text-muted">
				{message}
			</AlertDialog.Description>
			<div class="mt-6 flex justify-end gap-2">
				<AlertDialog.Cancel
					class="h-9 rounded-full px-4 text-sm font-semibold text-muted transition-colors hover:bg-surface hover:text-text"
				>
					{m.common_cancel()}
				</AlertDialog.Cancel>
				<AlertDialog.Action
					onclick={() => {
						open = false;
						onconfirm();
					}}
					class="h-9 rounded-full bg-danger/15 px-4 text-sm font-semibold text-danger transition-colors hover:bg-danger/25"
				>
					{confirmLabel}
				</AlertDialog.Action>
			</div>
		</AlertDialog.Content>
	</AlertDialog.Portal>
</AlertDialog.Root>
