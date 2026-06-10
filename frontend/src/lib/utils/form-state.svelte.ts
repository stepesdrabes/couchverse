/**
 * Dirty-state tracking for admin forms: snapshot the form's fields, compare
 * against a baseline captured after load/save. Save buttons stay disabled
 * until something actually changed, and async loads can check `dirty` before
 * overwriting in-progress edits.
 */
export class FormState<T> {
	#snapshot: () => T;
	#baseline = $state<string | null>(null);

	readonly dirty = $derived.by(
		() => this.#baseline !== null && JSON.stringify(this.#snapshot()) !== this.#baseline
	);

	constructor(snapshot: () => T) {
		this.#snapshot = snapshot;
	}

	/** true once a baseline has been captured (i.e. the form data loaded) */
	get loaded() {
		return this.#baseline !== null;
	}

	/** capture the current field values as the clean baseline */
	reset() {
		this.#baseline = JSON.stringify(this.#snapshot());
	}
}
