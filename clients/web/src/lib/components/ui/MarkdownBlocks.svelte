<script lang="ts">
	import type { Block } from '$lib/generated/core';
	import MarkdownBlocks from './MarkdownBlocks.svelte';
	import MarkdownInlines from './MarkdownInlines.svelte';

	let { blocks }: { blocks: Block[] } = $props();
</script>

<!-- The tree does not say whether a list is tight, so an item's first paragraph always sits
     directly in the <li>, as a tight list (the usual one in a bio) always rendered. -->
{#snippet item(blocks: Block[])}
	{@const [first, ...rest] = blocks}
	<li>
		{#if first?.type === 'paragraph'}
			<MarkdownInlines inlines={first.content} /><MarkdownBlocks blocks={rest} />
		{:else}
			<MarkdownBlocks {blocks} />
		{/if}
	</li>
{/snippet}

{#each blocks as block, i (i)}
	{#if block.type === 'paragraph'}
		<p><MarkdownInlines inlines={block.content} /></p>
	{:else if block.type === 'heading'}
		<svelte:element this={`h${block.content.level}`}>
			<MarkdownInlines inlines={block.content.inlines} />
		</svelte:element>
	{:else if block.type === 'quote'}
		<blockquote><MarkdownBlocks blocks={block.content} /></blockquote>
	{:else if block.type === 'list' && block.content.start === undefined}
		<ul>
			{#each block.content.items as { blocks }, j (j)}
				{@render item(blocks)}
			{/each}
		</ul>
	{:else if block.type === 'list'}
		<ol start={block.content.start}>
			{#each block.content.items as { blocks }, j (j)}
				{@render item(blocks)}
			{/each}
		</ol>
	{:else if block.type === 'code'}
		<pre><code>{block.content}</code></pre>
	{:else if block.type === 'rule'}
		<hr />
	{:else if block.type === 'table'}
		<table>
			<thead>
				<tr>
					{#each block.content.header as cell, j (j)}
						<th><MarkdownInlines inlines={cell.inlines} /></th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each block.content.rows as row, j (j)}
					<tr>
						{#each row.cells as cell, k (k)}
							<td><MarkdownInlines inlines={cell.inlines} /></td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
{/each}
