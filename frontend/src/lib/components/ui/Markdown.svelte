<script lang="ts">
	import { renderMarkdown } from '$lib/utils/markdown';

	// Renders user-authored markdown. Safe to {@html} because the shared
	// markdown-it instance parses with raw HTML disabled and rejects unsafe link
	// protocols, so the output can only contain the tags it generated itself.
	let { source, class: cls = '' }: { source: string; class?: string } = $props();

	const html = $derived(renderMarkdown(source));
</script>

<div class="markdown {cls}">
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html html}
</div>

<style lang="scss">
	.markdown {
		font-size: 0.875rem;
		line-height: 1.65;
		color: var(--color-muted);
		overflow-wrap: anywhere;

		:global(> :first-child) {
			margin-top: 0;
		}

		:global(> :last-child) {
			margin-bottom: 0;
		}

		:global(p) {
			margin: 0.6em 0;
		}

		// bios sit inside cards, so headings stay close to body size and lean on
		// weight and colour instead of scale
		:global(h1),
		:global(h2),
		:global(h3),
		:global(h4),
		:global(h5),
		:global(h6) {
			margin: 1.1em 0 0.4em;
			font-weight: 700;
			color: var(--color-text);
		}

		:global(h1) {
			font-size: 1.15em;
		}

		:global(h2) {
			font-size: 1.05em;
		}

		:global(strong) {
			font-weight: 700;
			color: var(--color-text);
		}

		:global(em) {
			font-style: italic;
		}

		:global(a) {
			color: var(--color-accent);
			text-decoration: underline;
			text-underline-offset: 2px;
		}

		:global(ul),
		:global(ol) {
			margin: 0.6em 0;
			padding-left: 1.25em;
		}

		:global(ul) {
			list-style: disc;
		}

		:global(ol) {
			list-style: decimal;
		}

		:global(li) {
			margin: 0.2em 0;
		}

		:global(blockquote) {
			margin: 0.8em 0;
			padding-left: 0.9em;
			border-left: 2px solid var(--color-accent);
			color: var(--color-faint);
		}

		:global(code) {
			padding: 0.1em 0.35em;
			border-radius: 6px;
			background: var(--color-surface-2);
			font-family: ui-monospace, 'SF Mono', Menlo, monospace;
			font-size: 0.9em;
			color: var(--color-text);
		}

		:global(pre) {
			margin: 0.8em 0;
			padding: 0.8em 1em;
			overflow-x: auto;
			border-radius: var(--radius-input);
			background: var(--color-surface-2);

			:global(code) {
				padding: 0;
				background: none;
			}
		}

		:global(hr) {
			margin: 1.2em 0;
			border: 0;
			border-top: 1px solid var(--color-edge);
		}
	}
</style>
