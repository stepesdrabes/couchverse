<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import { PAIRING_CODE_LENGTH, formatPairingCode, pairingLetters } from '../pairing';

	type Props = Omit<HTMLInputAttributes, 'value'> & {
		/** the code the way the device shows it, XXXX-XXXX (partial while typing) */
		value?: string;
		invalid?: boolean;
		input?: HTMLInputElement;
		/** the last letter went in, typed or pasted */
		oncomplete?: () => void;
	};

	let {
		value = $bindable(''),
		invalid = false,
		input = $bindable(),
		oncomplete,
		class: cls = '',
		...rest
	}: Props = $props();

	// The cells only picture one real text field laid over them, so typing, pasting
	// "wdjb mjht", autofill, on-screen keyboards, the TV remote and screen readers
	// all deal with a plain input.

	const letters = $derived(pairingLetters(value));
	const slots = Array.from({ length: PAIRING_CODE_LENGTH }, (_, i) => i);
	const cells: HTMLElement[] = [];

	let focused = $state(false);
	let selStart = $state(0);
	let selEnd = $state(0);

	const ranged = $derived(selStart !== selEnd);
	const caret = $derived(Math.min(selStart, PAIRING_CODE_LENGTH - 1));

	function tone(i: number): string {
		const active = focused && !ranged && i === caret;
		const fill =
			focused && ranged && i >= selStart && i < selEnd
				? 'bg-accent/25'
				: i < letters.length
					? 'bg-surface-2'
					: 'bg-surface';
		if (invalid) return `${fill} border-danger ${active ? 'ring-4 ring-danger/20' : ''}`;
		if (active) return `${fill} border-accent ring-4 ring-accent/20`;
		return `${fill} ${focused ? 'border-faint/70' : 'border-edge'}`;
	}

	function track() {
		if (!input) return;
		selStart = input.selectionStart ?? letters.length;
		selEnd = input.selectionEnd ?? letters.length;
	}

	function oninput(e: Event & { currentTarget: HTMLInputElement }) {
		const el = e.currentTarget;
		const at = pairingLetters(el.value.slice(0, el.selectionStart ?? el.value.length)).length;
		const next = pairingLetters(el.value);
		const completed = next !== letters && next.length === PAIRING_CODE_LENGTH;
		el.value = next;
		el.setSelectionRange(at, at);
		value = formatPairingCode(next);
		track();
		if (completed) oncomplete?.();
	}

	// The invisible text does not line up with the cells, so a click puts the caret
	// at the cell under the pointer rather than where the browser placed it.
	function onclick(e: MouseEvent & { currentTarget: HTMLInputElement }) {
		if (e.currentTarget.selectionStart !== e.currentTarget.selectionEnd) return;
		const hit = cells.findIndex((cell) => e.clientX < cell.getBoundingClientRect().right);
		const at = Math.min(hit < 0 ? PAIRING_CODE_LENGTH : hit, letters.length);
		e.currentTarget.setSelectionRange(at, at);
		track();
	}
</script>

<div class="relative {cls}">
	<div class="flex items-center justify-center gap-1.5 sm:gap-2" aria-hidden="true">
		{#each slots as i (i)}
			{#if i === PAIRING_CODE_LENGTH / 2}
				<span class="mx-0.5 h-0.5 w-2.5 shrink-0 rounded-full bg-faint/60 sm:mx-1 sm:w-3"></span>
			{/if}
			<span
				bind:this={cells[i]}
				class="flex aspect-[4/5] max-w-12 min-w-0 flex-1 items-center justify-center
					rounded-input border font-mono text-2xl font-bold transition-[border-color,box-shadow,background-color]
					duration-150 sm:text-3xl {tone(i)}"
			>
				{letters[i] ?? ''}
				{#if focused && !ranged && i === caret && i >= letters.length}
					<span class="caret"></span>
				{/if}
			</span>
		{/each}
	</div>

	<input
		bind:this={input}
		value={letters}
		{oninput}
		{onclick}
		onkeyup={track}
		onselect={track}
		onfocus={() => {
			focused = true;
			track();
		}}
		onblur={() => (focused = false)}
		type="text"
		inputmode="text"
		autocomplete="one-time-code"
		autocapitalize="characters"
		autocorrect="off"
		spellcheck="false"
		enterkeyhint="go"
		aria-invalid={invalid}
		class="absolute inset-0 size-full cursor-text appearance-none border-0 bg-transparent p-0
			text-base text-transparent caret-transparent outline-none selection:bg-transparent"
		{...rest}
	/>
</div>

<style lang="scss">
	.caret {
		width: 2px;
		height: 1.1em;
		border-radius: 1px;
		background: var(--color-accent);
		animation: blink 1.1s step-end infinite;
	}

	@keyframes blink {
		50% {
			opacity: 0;
		}
	}
</style>
