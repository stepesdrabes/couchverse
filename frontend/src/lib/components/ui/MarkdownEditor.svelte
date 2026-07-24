<script lang="ts">
	import { Bold, Code, Italic, Link2, List, Quote, Strikethrough } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import Markdown from './Markdown.svelte';
	import Tooltip from './Tooltip.svelte';
	import * as m from '$lib/paraglide/messages';

	// A small editor composed from the app's own primitives rather than a
	// drop-in library: a third-party editor ships its own CSS and would fight the
	// design system for the sake of a bio field.
	let {
		value = $bindable(''),
		maxlength = 2000,
		rows = 7,
		placeholder = ''
	}: {
		value?: string;
		maxlength?: number;
		rows?: number;
		placeholder?: string;
	} = $props();

	let textarea = $state<HTMLTextAreaElement>();
	let mode = $state<'write' | 'preview'>('write');

	/**
	 * Wraps the selection, or inserts the marker pair and puts the caret between
	 * them when nothing is selected. Toggling off again is deliberate: clicking
	 * bold twice should undo it, not nest it.
	 */
	function wrap(before: string, after = before) {
		const el = textarea;
		if (!el) return;
		const { selectionStart: start, selectionEnd: end } = el;
		const selected = value.slice(start, end);
		const already =
			value.slice(start - before.length, start) === before &&
			value.slice(end, end + after.length) === after;

		if (already) {
			value = value.slice(0, start - before.length) + selected + value.slice(end + after.length);
			queueCaret(el, start - before.length, end - before.length);
			return;
		}
		value = value.slice(0, start) + before + selected + after + value.slice(end);
		queueCaret(el, start + before.length, end + before.length);
	}

	/** prefixes every selected line, for lists and quotes */
	function prefixLines(marker: string) {
		const el = textarea;
		if (!el) return;
		const { selectionStart: start, selectionEnd: end } = el;
		const lineStart = value.lastIndexOf('\n', start - 1) + 1;
		const block = value.slice(lineStart, end);
		const lines = block.split('\n');
		const stripped = lines.every((line) => line.startsWith(marker));
		const next = lines
			.map((line) => (stripped ? line.slice(marker.length) : marker + line))
			.join('\n');
		value = value.slice(0, lineStart) + next + value.slice(end);
		queueCaret(el, lineStart, lineStart + next.length);
	}

	// the value round-trips through the binding, so the caret has to be restored
	// after Svelte has written the new text back into the element
	function queueCaret(el: HTMLTextAreaElement, start: number, end: number) {
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(start, end);
		});
	}

	const tools = $derived([
		{ key: 'bold', icon: Bold, label: m.markdown_bold(), run: () => wrap('**') },
		{ key: 'italic', icon: Italic, label: m.markdown_italic(), run: () => wrap('_') },
		{ key: 'strike', icon: Strikethrough, label: m.markdown_strike(), run: () => wrap('~~') },
		{ key: 'code', icon: Code, label: m.markdown_code(), run: () => wrap('`') },
		{ key: 'link', icon: Link2, label: m.markdown_link(), run: () => wrap('[', '](https://)') },
		{ key: 'list', icon: List, label: m.markdown_list(), run: () => prefixLines('- ') },
		{ key: 'quote', icon: Quote, label: m.markdown_quote(), run: () => prefixLines('> ') }
	]);

	const remaining = $derived(maxlength - value.length);
</script>

<div class="overflow-hidden rounded-input border border-edge bg-surface focus-within:border-accent">
	<div class="flex items-center gap-0.5 border-b border-edge/70 px-1.5 py-1">
		{#each tools as tool (tool.key)}
			<Tooltip label={tool.label}>
				{#snippet trigger(props)}
					<button
						{...props}
						type="button"
						class="rounded-md p-1.5 text-muted transition-colors hover:bg-surface-2 hover:text-text
							disabled:opacity-40"
						disabled={mode === 'preview'}
						aria-label={tool.label}
						onclick={tool.run}
					>
						<tool.icon class="size-3.5" />
					</button>
				{/snippet}
			</Tooltip>
		{/each}

		<div class="ml-auto flex items-center gap-1 rounded-full bg-surface-2 p-0.5">
			{#each [{ v: 'write', l: m.markdown_write() }, { v: 'preview', l: m.markdown_preview() }] as tab (tab.v)}
				<button
					type="button"
					class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold transition-colors
						{mode === tab.v ? 'bg-accent-soft text-text' : 'text-muted hover:text-text'}"
					onclick={() => (mode = tab.v as 'write' | 'preview')}
				>
					{tab.l}
				</button>
			{/each}
		</div>
	</div>

	{#if mode === 'write'}
		<textarea
			bind:this={textarea}
			bind:value
			{rows}
			{maxlength}
			{placeholder}
			class="block w-full resize-y bg-transparent px-3.5 py-3 text-sm text-text
				placeholder:text-faint focus:outline-none"
		></textarea>
	{:else}
		<div class="min-h-[7rem] px-3.5 py-3" in:fly|global={{ y: 6, duration: 200 }}>
			{#if value.trim()}
				<Markdown source={value} />
			{:else}
				<p class="text-sm text-faint">{m.markdown_empty_preview()}</p>
			{/if}
		</div>
	{/if}

	<div class="flex items-center justify-between border-t border-edge/70 px-3.5 py-1.5">
		<span class="text-[11px] text-faint">{m.markdown_hint()}</span>
		<span class="text-[11px] tnum {remaining < 0 ? 'text-danger' : 'text-faint'}">{remaining}</span>
	</div>
</div>
