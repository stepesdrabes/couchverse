// The news (placeholder until the bulletin design lands): the top stories, each headline whole
// beside its outlet's name, and the credit line. The rows settle in the first seconds and then
// hold still, so most frames render as one capture.

import { div, rise } from '../core/dom';
import { fitText } from '../core/fit';
import type { NewsSpec } from '../spec';
import { bulletinBackdrop, bulletinHead, bulletinOut, bulletinSound, credit } from './bulletin';
import { timeline, type Template } from './shared';

// the list runs from under the header to the credit line (px); each row is the outlet's line
// and the headline under it
const LIST_HEIGHT = 660;
const ROW_GAP = 22;
const OUTLET_HEIGHT = 34;

export const news: Template<NewsSpec> = {
	version: 1,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 20;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'news');
		bulletinBackdrop(tl, root);
		bulletinHead(tl, root, 'News', 'Top stories', spec.asOf);

		const list = div('nw-list', root);
		const stories = spec.stories.slice(0, 5);
		// the stories share the list's height, a headline breaking onto lines before it shrinks
		const row = Math.floor((LIST_HEIGHT - ROW_GAP * (stories.length - 1)) / stories.length);
		const rows = stories.map((s, i) => {
			const r = div('nw-row', list);
			div('nw-rank tnum', r, String(i + 1));
			const text = div('nw-text', r);
			div('nw-outlet', text, s.outlet);
			const headline = div('nw-headline', text, s.headline);
			fitText(headline, {
				width: 1500,
				min: 34,
				lines: 3,
				leading: 1.12,
				height: row - OUTLET_HEIGHT
			});
			return r;
		});
		const at = 0.6;
		rise(tl, rows, at, { stagger: 0.14, dur: 0.8, from: '40px' });
		credit(tl, root, spec.credit, at + 0.4);
		bulletinOut(tl, root);
		bulletinSound(
			tl,
			sound,
			rows.map((_, i) => at + i * 0.14)
		);
		return tl;
	}
};
