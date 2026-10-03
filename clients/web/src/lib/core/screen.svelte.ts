import { untrack } from 'svelte';
import type { Surface } from '$lib/generated/core';
import { core } from '.';

/**
 * Opens the screen `surface()` names while the calling component is mounted (following it when
 * it changes, nothing while it is undefined) and reads its view. Call during component setup.
 * With `revalidate`, opening it reloads it even when the core holds it as fresh (see
 * `CoreRuntime.open`).
 */
export function useScreen<T>(
	surface: () => Surface | undefined,
	options: { revalidate?: boolean } = {}
) {
	$effect(() => {
		const screen = surface();
		if (screen) return untrack(() => core.open(screen, options));
	});
	const view = $derived.by(() => {
		const screen = surface();
		return screen && core.view<T>(screen);
	});
	return {
		get view() {
			return view;
		}
	};
}
