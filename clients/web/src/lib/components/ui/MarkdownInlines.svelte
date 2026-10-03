<script lang="ts">
	import type { Inline } from '$lib/generated/core';
	import MarkdownInlines from './MarkdownInlines.svelte';

	let { inlines }: { inlines: Inline[] } = $props();
</script>

{#each inlines as inline, i (i)}
	{#if inline.type === 'text'}
		{inline.content}
	{:else if inline.type === 'code'}
		<code>{inline.content}</code>
	{:else if inline.type === 'strong'}
		<strong><MarkdownInlines inlines={inline.content} /></strong>
	{:else if inline.type === 'emphasis'}
		<em><MarkdownInlines inlines={inline.content} /></em>
	{:else if inline.type === 'strike'}
		<s><MarkdownInlines inlines={inline.content} /></s>
	{:else if inline.type === 'link'}
		<!-- every link leaves for somewhere untrusted -->
		<a href={inline.content.href} target="_blank" rel="nofollow noopener noreferrer"
			><MarkdownInlines inlines={inline.content.children} /></a
		>
	{:else if inline.type === 'break'}
		<br />
	{/if}
{/each}
