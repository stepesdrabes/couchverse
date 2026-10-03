import { readFileSync } from 'node:fs';
import { render } from 'svelte/server';
import { expect, it } from 'vitest';
import { spawner } from '$lib/core/wasm';
import type { MarkdownDoc } from '$lib/generated/core';
import MarkdownBlocks from './MarkdownBlocks.svelte';

const wasm = new URL('../../core/pkg/couchverse_core_bg.wasm', import.meta.url);
const spawn = spawner(WebAssembly.compile(readFileSync(wasm)));

async function html(source: string) {
	const config = { platform: 'web', authMode: 'cookie', deviceName: '', locale: 'en' };
	const core = await spawn(JSON.stringify(config));
	const surface = JSON.stringify({ type: 'markdown', content: source });
	const doc: MarkdownDoc = JSON.parse(core.view(surface));
	// hydration markers aside
	return render(MarkdownBlocks, { props: { blocks: doc.blocks } }).body.replace(/<!--.*?-->/g, '');
}

it('renders a bio as the elements the markdown styles expect', async () => {
	const bio = [
		'# Hi there',
		'I watch **sci-fi**\nand _horror_.',
		'- one\n- two `x`\n  1. nested',
		'> quoted',
		'~~old~~ [ok](https://example.com)',
		'---',
		'```\nlet x = 1;\n```',
		'| a | b |\n|---|---|\n| 1 | 2 |'
	].join('\n\n');

	expect(await html(bio)).toBe(
		'<h1>Hi there</h1>' +
			'<p>I watch <strong>sci-fi</strong><br/>and <em>horror</em>.</p>' +
			'<ul><li>one</li><li>two <code>x</code><ol start="1"><li>nested</li></ol></li></ul>' +
			'<blockquote><p>quoted</p></blockquote>' +
			'<p><s>old</s> <a href="https://example.com" target="_blank" rel="nofollow noopener noreferrer">ok</a></p>' +
			'<hr/>' +
			'<pre><code>let x = 1;</code></pre>' +
			'<table><thead><tr><th>a</th><th>b</th></tr></thead>' +
			'<tbody><tr><td>1</td><td>2</td></tr></tbody></table>'
	);
});

it('shows raw HTML as text and drops unsafe links', async () => {
	expect(await html('<img src=x onerror=alert(1)> [bad](javascript:alert(1))')).toBe(
		'<p>&lt;img src=x onerror=alert(1)> bad</p>'
	);
});
