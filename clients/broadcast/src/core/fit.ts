// Spec text (titles, taglines, episode names) fitted to its box: the design's size when it fits,
// else shrunk to the role's minimum on one line, else broken onto balanced lines, and shrunk
// further only when even that is not enough. A pure function of the text, the box and the
// bundled fonts: it reads layout once fonts are in, before the text's own animations exist,
// and works on what the DOM helpers build (plain text, `words`, `chars`), so per-word and
// per-character animations keep their targets.

/** The title-safe width: the stage less 96 px each side. */
export const SAFE_WIDTH = 1920 - 2 * 96;

export interface FitBox {
	/** The widest a line may be, px, at the largest scale the text's animations reach. */
	width: number;
	/** The smallest size (px) the text shrinks to on one line before it breaks. */
	min: number;
	/** The most lines it may break onto. */
	lines?: number;
	/** The tallest the text may be, px. */
	height?: number;
	/** The line height once it breaks, as a ratio of the size (default: the role's own). */
	leading?: number;
}

/** A word on one line: where it starts and ends, px. */
export interface Word {
	text: string;
	left: number;
	right: number;
}

// a line that ends on a colon or a dash, or starts with "and" or "the", reads as a phrase: worth
// a slightly wider line; a one-letter word ending a line is a Czech typesetting error
const AFTER = /[:\u2013\u2014]$/;
const BEFORE = new Set(['and', 'the']);
const ALONE = /^[\u2013\u2014\-&:·/|]+$/;
const LETTER = /^\p{L}$/u;
const BONUS_AFTER = 0.2;
const BONUS_BEFORE = 0.08;
const PENALTY_LETTER = 0.25;

/**
 * Where to break `words` onto `lines` lines, as the indexes of the words that start a new line:
 * the widest line as narrow as possible (the largest type), a break after a colon or a dash or
 * before "and" or "the" when it costs little width, and never a widow (one word alone on the
 * last line). Null when there are fewer words than lines or every way has a line wider than
 * `max`.
 */
export function breakLines(words: Word[], lines: number, max = Infinity): number[] | null {
	const n = words.length;
	if (lines < 1 || lines > n) return null;
	const width = (a: number, b: number) => words[b - 1].right - words[a].left;
	if (lines === 1) return width(0, n) <= max ? [] : null;
	const ways: { starts: number[]; widths: number[] }[] = [];
	const choose = (starts: number[]) => {
		const k = starts.length;
		if (k === lines) {
			const ends = [...starts.slice(1), n];
			ways.push({ starts: starts.slice(1), widths: starts.map((s, i) => width(s, ends[i])) });
			return;
		}
		const from = starts[k - 1] + 1;
		// the lines still to come each need a word; the last needs two unless every line has one
		const last = n - (lines - k) - (n > lines ? 1 : 0);
		for (let s = from; s <= last; s++) {
			if (ALONE.test(words[s].text)) continue;
			choose([...starts, s]);
		}
	};
	choose([0]);
	const fits = ways.filter((w) => Math.max(...w.widths) <= max);
	if (!fits.length) return null;
	const narrowest = Math.min(...fits.map((w) => Math.max(...w.widths)));
	let best: number[] | null = null;
	let bestCost = Infinity;
	for (const w of fits) {
		const widest = Math.max(...w.widths);
		let cost = widest / narrowest;
		for (const s of w.starts) {
			if (AFTER.test(words[s - 1].text)) cost -= BONUS_AFTER;
			if (BEFORE.has(words[s].text.toLowerCase())) cost -= BONUS_BEFORE;
			if (LETTER.test(words[s - 1].text)) cost += PENALTY_LETTER;
		}
		// evenly long lines among ways that are otherwise alike
		cost += (0.05 * w.widths.reduce((a, x) => a + (1 - x / widest) ** 2, 0)) / w.widths.length;
		if (cost < bestCost - 1e-9) {
			bestCost = cost;
			best = w.starts;
		}
	}
	return best;
}

interface Segment {
	node: Text;
	start: number;
	end: number;
}

/** Where a line may break: spaces in text, `chars`' spacers, breaks already made. */
type Gap = Segment | { el: Element };

/** The text's words (as text segments) and the gap after each but the last. */
function tokens(root: HTMLElement) {
	const words: Segment[][] = [];
	const gaps: Gap[] = [];
	let word: Segment[] | null = null;
	let gap: Gap | null = null;
	const space = (g: Gap) => {
		if (word) words.push(word);
		word = null;
		gap ??= g;
	};
	const text = (s: Segment) => {
		if (!word) {
			if (words.length && gap) gaps.push(gap);
			gap = null;
			word = [];
		}
		word.push(s);
	};
	const walk = (node: Node) => {
		if (node instanceof Text) {
			const t = node.data;
			let i = 0;
			while (i < t.length) {
				let j = i;
				const blank = t[i] === ' ';
				while (j < t.length && (t[j] === ' ') === blank) j++;
				if (blank) space({ node, start: i, end: j });
				else text({ node, start: i, end: j });
				i = j;
			}
		} else if (node instanceof Element) {
			if (node.classList.contains('sp') || node.tagName === 'BR') space({ el: node });
			else node.childNodes.forEach(walk);
		}
	};
	root.childNodes.forEach(walk);
	if (word) words.push(word);
	return { words, gaps };
}

/** How much the element is scaled on screen (a fitted preview stage), measured by a probe. */
function scaleOf(el: HTMLElement) {
	const probe = document.createElement('span');
	probe.style.cssText = 'position: absolute; width: 1000px; height: 0';
	el.appendChild(probe);
	const k = probe.getBoundingClientRect().width / 1000;
	probe.remove();
	return k > 0 ? k : 1;
}

// layout is in 1/64 px: rounding there undoes a scaled stage's float error, so a preview fits
// exactly as the render does
const unit = (x: number, k: number) => Math.round((x / k) * 64) / 64;

function measure(words: Segment[][], k: number): Word[] {
	const range = document.createRange();
	return words.map((segs) => {
		let left = Infinity;
		let right = -Infinity;
		let text = '';
		for (const s of segs) {
			range.setStart(s.node, s.start);
			range.setEnd(s.node, s.end);
			const r = range.getBoundingClientRect();
			left = Math.min(left, r.left);
			right = Math.max(right, r.right);
			text += s.node.data.slice(s.start, s.end);
		}
		return { text, left: unit(left, k), right: unit(right, k) };
	});
}

/** Break the text before the words at `starts`: the gap before each becomes a `<br>`. */
function breakAt(gaps: Gap[], starts: number[]) {
	for (const s of [...starts].sort((a, b) => b - a)) {
		const g = gaps[s - 1];
		const br = document.createElement('br');
		if ('el' in g) g.el.replaceWith(br);
		else {
			g.node.splitText(g.end);
			g.node.splitText(g.start).replaceWith(br);
		}
	}
}

/** The widest line and the height of the text as laid out now, px. */
function extent(el: HTMLElement, starts: number[], k: number) {
	const words = measure(tokens(el).words, k);
	const ends = [...starts, words.length];
	let widest = 0;
	ends.forEach((e, i) => {
		const s = i ? ends[i - 1] : 0;
		widest = Math.max(widest, words[e - 1].right - words[s].left);
	});
	return { widest, height: unit(el.getBoundingClientRect().height, k) };
}

/** Fit `el`'s text to `box` (see the top of this file). */
export function fitText(el: HTMLElement, box: FitBox) {
	const { words: segs, gaps } = tokens(el);
	if (!segs.length) return;
	const k = scaleOf(el);
	const style = getComputedStyle(el);
	// measured on one line, whatever the box would wrap
	const wrap = el.style.whiteSpace;
	const pre = style.whiteSpace.startsWith('pre');
	if (!pre) el.style.whiteSpace = 'nowrap';
	const words = measure(segs, k);
	const n = words.length;
	const natural = words[n - 1].right - words[0].left;
	if (natural <= box.width) {
		el.style.whiteSpace = wrap;
		return;
	}
	const s0 = parseFloat(style.fontSize);
	const own = parseFloat(style.lineHeight) / s0 || unit(el.getBoundingClientRect().height, k) / s0;
	const ratio = (lines: number) => (lines > 1 ? (box.leading ?? own) : own);
	const cap = (lines: number) =>
		box.height ? Math.floor(box.height / (lines * ratio(lines))) : Infinity;
	const widest = (starts: number[]) => {
		const ends = [...starts, n];
		return Math.max(...ends.map((e, i) => words[e - 1].right - words[i ? ends[i - 1] : 0].left));
	};
	const plan = (lines: number, max = Infinity) => {
		const starts = breakLines(words, lines, max);
		if (!starts) return null;
		const size = Math.min(s0, Math.floor((s0 * box.width) / widest(starts)), cap(lines));
		return { starts, size: Math.max(1, size) };
	};
	const most = Math.min(box.lines ?? 1, n);
	let best = {
		starts: [] as number[],
		size: Math.max(1, Math.min(Math.floor((s0 * box.width) / natural), cap(1)))
	};
	for (let lines = 2; best.size < box.min && lines <= most; lines++) {
		const p = plan(lines, (box.width * s0) / box.min);
		if (p && p.size >= box.min) best = p;
	}
	// not even the most lines fit at the minimum: whichever count gives the largest type
	for (let lines = 2; best.size < box.min && lines <= most; lines++) {
		const p = plan(lines);
		if (p && p.size > best.size) best = p;
	}
	breakAt(gaps, best.starts);
	if (best.starts.length && box.leading) el.style.lineHeight = String(box.leading);
	// widths scale with the size up to rounding: step down until the text really fits
	let size = best.size;
	for (;;) {
		el.style.fontSize = `${size}px`;
		const e = extent(el, best.starts, k);
		if (size <= 1 || (e.widest <= box.width && (!box.height || e.height <= box.height))) break;
		size--;
	}
}
