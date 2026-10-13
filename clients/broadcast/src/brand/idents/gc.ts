// A motion-blurred frame reads its canvas back once per sub-frame, up to 64 times, and V8 leaves
// those buffers to a later collection: gigabytes of them within a few frames, more than the
// render sidecar's 1 GB container holds. The sidecar starts Chromium with --expose-gc, so a
// render collects them as it goes; elsewhere (the admin preview) there is no `gc` to call.

/** How much garbage may pile up before it is collected. */
const LIMIT = 64 * 2 ** 20;

let pending = 0;

/** Count `bytes` of garbage just made, and collect once enough has piled up (where we may). */
export function collectGarbage(bytes: number) {
	pending += bytes;
	if (pending < LIMIT) return;
	pending = 0;
	(globalThis as { gc?: () => void }).gc?.();
}
