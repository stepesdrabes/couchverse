<script lang="ts">
	import { toast } from 'svelte-sonner';
	import type { NoticesView, Surface } from '$lib/generated/core';
	import * as m from '$lib/paraglide/messages';
	import { core } from '.';

	// Renders nothing: the core's notices (something failed with no screen of its own, like a
	// My List change the server refused) become toasts, dismissed in the core when they close.
	const NOTICES: Surface = { type: 'notices' };

	$effect(() => core.watch(NOTICES));

	// the notices already up as toasts: bookkeeping, never rendered
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const shown = new Set<number>();

	$effect(() => {
		for (const { id, code } of core.view<NoticesView>(NOTICES)?.notices ?? []) {
			if (shown.has(id)) continue;
			shown.add(id);
			const dismiss = () => void core.send({ type: 'noticeDismissed', content: { id } });
			toast.error(message(code), { onDismiss: dismiss, onAutoClose: dismiss });
		}
	});

	function message(code: string): string {
		switch (code) {
			case 'watchlist_failed':
				return m.catalog_list_update_failed();
			case 'visibility_failed':
				return m.profiles_privacy_failed();
			case 'couch_disabled':
				return m.couch_start_failed();
			case 'couch_nothing_playing':
				return m.couch_start_from_video();
			default:
				return m.error_page_title();
		}
	}
</script>
