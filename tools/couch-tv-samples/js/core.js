// Shared kernel for the Couch TV templates: the Timeline (picture and sound cues on one
// clock), motion curves from the design tokens, the accent palette (a port of the web's
// theme.ts), artwork framing and text helpers.

export const W = 1920;
export const H = 1080;

// tokens.json motion.curve plus the broadcast additions
export const EASE = {
	standard: 'cubic-bezier(0.25, 0.1, 0.25, 1)',
	enter: 'cubic-bezier(0.16, 1, 0.3, 1)',
	exit: 'cubic-bezier(0.7, 0, 0.84, 0)',
	inOut: 'cubic-bezier(0.65, 0, 0.35, 1)',
	sine: 'cubic-bezier(0.37, 0, 0.63, 1)',
	out: 'cubic-bezier(0.22, 1, 0.36, 1)',
	in: 'cubic-bezier(0.55, 0, 1, 0.45)',
	snap: 'cubic-bezier(0.05, 0.9, 0.1, 1)',
	pop: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
	linear: 'linear'
};

// tokens.json motion.spring
export const SPRING = {
	snappy: { response: 0.3, damping: 0.86 },
	smooth: { response: 0.45, damping: 0.9 },
	bouncy: { response: 0.5, damping: 0.68 }
};

/** A damped spring from 0 to 1 sampled as keyframe progress values, plus its settle time. */
export function spring({ response, damping }, samples = 28) {
	const w0 = (2 * Math.PI) / response;
	const zeta = damping;
	const wd = w0 * Math.sqrt(1 - zeta * zeta);
	const settle = Math.log(1000) / (zeta * w0);
	const values = [];
	for (let i = 0; i <= samples; i++) {
		const t = (i / samples) * settle;
		const env = Math.exp(-zeta * w0 * t);
		values.push(1 - env * (Math.cos(wd * t) + ((zeta * w0) / wd) * Math.sin(wd * t)));
	}
	values[samples] = 1;
	return { values, duration: settle };
}

/**
 * One piece's clock. Animations are Web Animations created paused with their start time
 * folded into `delay`, so seeking is setting `currentTime` and playing is giving every
 * animation the same `startTime`, derived from the audio clock. Sounds are callbacks
 * scheduled against the same zero.
 */
export class Timeline {
	constructor(root, duration) {
		this.root = root;
		this.duration = duration;
		this.anims = [];
		this.sounds = [];
		this.ticks = [];
		this.cues = [];
		this._props = new WeakMap();
		this.docStart = null;
	}

	/**
	 * Animate `target` from `at` for `dur` seconds. The first animation of a property on an
	 * element fills both ways (it owns the element's state before its cue); later ones only
	 * fill forwards, so cues stack in time order without fighting.
	 */
	add(target, frames, at, dur, ease = 'enter', opts = {}) {
		if (!target) return null;
		const props = new Set();
		for (const f of frames) for (const k of Object.keys(f)) if (k !== 'offset' && k !== 'easing') props.add(k);
		let seen = this._props.get(target);
		if (!seen) this._props.set(target, (seen = new Set()));
		const again = [...props].some((p) => seen.has(p));
		props.forEach((p) => seen.add(p));
		const a = target.animate(frames, {
			duration: Math.max(1, dur * 1000),
			delay: at * 1000,
			fill: opts.fill || (again ? 'forwards' : 'both'),
			easing: EASE[ease] || ease,
			iterations: opts.iterations || 1,
			direction: opts.direction || 'normal'
		});
		a.pause();
		this.anims.push(a);
		return a;
	}

	/** Visible only within [a, b): hard cuts. Uses visibility, so opacity stays free. */
	show(target, a, b) {
		target.style.visibility = 'hidden';
		return this.add(target, [{ visibility: 'visible' }, { visibility: 'visible' }], a, b - a, 'linear', {
			fill: 'none'
		});
	}

	/** A sound cue: fn(when, bus, skip) with `when` in AudioContext time. `dur` lets a long
	 *  sound (a bed, a drone) start part-way through when playback starts inside it. */
	sound(at, fn, dur = 0) {
		this.sounds.push({ at, fn, dur });
	}

	/** Called every frame with the timeline time (text that changes: clocks, counters). */
	tick(fn) {
		this.ticks.push(fn);
	}

	cue(name, at) {
		this.cues.push({ name, at });
		return at;
	}

	seek(t) {
		this.docStart = null;
		for (const a of this.anims) {
			a.pause();
			a.currentTime = t * 1000;
		}
		this.update(t);
	}

	/** Start at document time `docStart` (ms) from timeline offset `offset` (s). */
	play(docStart, offset = 0) {
		this.docStart = docStart - offset * 1000;
		for (const a of this.anims) a.startTime = this.docStart;
	}

	scheduleSounds(ctxStart, offset, bus) {
		for (const s of this.sounds) {
			const inside = s.dur > 0 && offset > s.at && offset < s.at + s.dur;
			if (s.at < offset - 0.03 && !inside) continue;
			const skip = Math.max(0, offset - s.at);
			try {
				s.fn(ctxStart + Math.max(0, s.at - offset), bus, skip);
			} catch (err) {
				console.warn('sound cue failed', err);
			}
		}
	}

	time(now = document.timeline.currentTime) {
		if (this.docStart == null) return null;
		return (now - this.docStart) / 1000;
	}

	update(t) {
		for (const fn of this.ticks) fn(t);
	}

	pause() {
		for (const a of this.anims) a.pause();
	}

	cancel() {
		for (const a of this.anims) a.cancel();
		this.anims = [];
	}

	cueAt(t) {
		let cur = null;
		for (const c of this.cues) if (c.at <= t + 1e-3 && (!cur || c.at >= cur.at)) cur = c;
		return cur;
	}
}

/**
 * One animation per element for the whole piece from a list of moves ({at, dur, to, ease}):
 * the value holds between moves, so a move of 0 s is a hard cut.
 */
export function motion(tl, target, D, v0, moves, fmt) {
	const frames = [{ offset: 0, ...fmt(v0) }];
	let cur = v0;
	for (const mv of [...moves].sort((a, b) => a.at - b.at)) {
		const a = Math.max(0, mv.at);
		const z = a + Math.max(0.001, mv.dur || 0);
		frames.push({ offset: a / D, easing: EASE[mv.ease] || mv.ease || 'linear', ...fmt(cur) });
		frames.push({ offset: Math.min(1, z / D), ...fmt(mv.to) });
		cur = mv.to;
	}
	frames.push({ offset: 1, ...fmt(cur) });
	for (let i = 1; i < frames.length; i++) frames[i].offset = Math.min(1, Math.max(frames[i].offset, frames[i - 1].offset));
	return tl.add(target, frames, 0, D, 'linear');
}

/**
 * Visible over the union of windows [[a, b], ...]: one opacity animation of hard steps, so
 * cuts stay on the compositor with the transforms they go with (visibility would wait for
 * the main thread and can land a frame late under load).
 */
export function windows(tl, target, list) {
	const D = tl.duration;
	const ws = list.filter(([a, b]) => b > a).sort((x, y) => x[0] - y[0]);
	const merged = [];
	for (const [a, b] of ws) {
		const last = merged[merged.length - 1];
		if (last && a <= last[1]) last[1] = Math.max(last[1], b);
		else merged.push([a, b]);
	}
	const at = (t) => Math.min(1, Math.max(0, t / D));
	const on0 = merged.length && merged[0][0] <= 0;
	const frames = [{ offset: 0, opacity: on0 ? 1 : 0 }];
	for (const [a, b] of merged) {
		if (a > 0) frames.push({ offset: at(a), opacity: 0 }, { offset: at(a), opacity: 1 });
		if (b < D) frames.push({ offset: at(b), opacity: 1 }, { offset: at(b), opacity: 0 });
	}
	frames.push({ offset: 1, opacity: frames[frames.length - 1].opacity });
	tl.add(target, frames, 0, D, 'linear');
}

/** Subtract windows [x, y] from windows. */
export function minus(list, cuts) {
	let out = list;
	for (const [x, y] of cuts) {
		const next = [];
		for (const [a, b] of out) {
			if (y <= a || x >= b) next.push([a, b]);
			else {
				if (a < x) next.push([a, x]);
				if (y < b) next.push([y, b]);
			}
		}
		out = next;
	}
	return out;
}

// --- DOM ------------------------------------------------------------------------------

export function el(tag, cls, parent, text) {
	const e = document.createElement(tag);
	if (cls) e.className = cls;
	if (text != null) e.textContent = text;
	if (parent) parent.appendChild(e);
	return e;
}

export function css(e, styles) {
	Object.assign(e.style, styles);
	return e;
}

/** Text split into characters (inline-blocks). Spaces become fixed-width spacers. */
export function chars(parent, text, cls) {
	const root = el('span', cls, parent);
	const items = [];
	for (const c of text) {
		if (c === ' ') {
			el('span', 'sp', root);
			continue;
		}
		items.push(el('span', 'ch', root, c));
	}
	return { root, items };
}

/** Text split into words, each inside its own clip mask (for rise-from-the-edge reveals). */
export function words(parent, text, cls) {
	const root = el('span', cls, parent);
	const items = [];
	text.split(' ').forEach((w, i) => {
		if (i) root.appendChild(document.createTextNode(' '));
		const m = el('span', 'mask', root);
		items.push(el('span', 'w', m, w));
	});
	return { root, items };
}

export function rise(tl, items, at, { stagger = 0.07, dur = 0.9, from = '105%', ease = 'enter', fade = true } = {}) {
	items.forEach((it, i) => {
		const f = fade
			? [
					{ transform: `translateY(${from})`, opacity: 0 },
					{ transform: 'translateY(0)', opacity: 1 }
				]
			: [{ transform: `translateY(${from})` }, { transform: 'translateY(0)' }];
		tl.add(it, f, at + i * stagger, dur, ease);
	});
	return at + (items.length - 1) * stagger + dur;
}

export function fadeUp(tl, e, at, { dur = 0.8, y = 18, ease = 'enter' } = {}) {
	tl.add(
		e,
		[
			{ opacity: 0, transform: `translateY(${y}px)` },
			{ opacity: 1, transform: 'translateY(0)' }
		],
		at,
		dur,
		ease
	);
}

export function fadeOut(tl, e, at, { dur = 0.45, y = -12, x = 0, scale = 1, ease = 'exit' } = {}) {
	tl.add(
		e,
		[
			{ opacity: 1, transform: 'translate(0, 0) scale(1)' },
			{ opacity: 0, transform: `translate(${x}px, ${y}px) scale(${scale})` }
		],
		at,
		dur,
		ease
	);
}

/**
 * A light sweep across text: a window moving over a bright copy of the text that moves the
 * other way, so the copy stays registered. Two transforms, no paint per frame.
 */
export function glint(tl, host, at, dur = 0.9, { band = 0.32, ease = 'inOut' } = {}) {
	host.classList.add('glint-host');
	const w = host.offsetWidth;
	const bw = Math.max(80, w * band);
	const wrap = el('span', 'glint-wrap', host);
	const win = el('span', 'glint-band', wrap);
	css(win, { width: bw + 'px' });
	const copy = host.cloneNode(true);
	copy.querySelector('.glint-wrap')?.remove();
	copy.className = (host.className + ' glint-copy').replace('glint-host', '');
	copy.removeAttribute('style');
	css(copy, { width: w + 'px', position: 'absolute', margin: '0' });
	win.appendChild(copy);
	tl.add(win, [{ transform: `translateX(${-bw}px)` }, { transform: `translateX(${w}px)` }], at, dur, ease);
	tl.add(copy, [{ transform: `translateX(${bw}px)` }, { transform: `translateX(${-w}px)` }], at, dur, ease);
	tl.show(win, at, at + dur);
	return win;
}

// --- artwork framing ---------------------------------------------------------------------

/**
 * An image laid out to cover a `view`, with `tf(rect)` giving the transform that fills the
 * view with `rect` (normalized [x, y, w, h] of the image), clamped so no edge shows.
 */
export function framed(parent, src, dims, view = { w: W, h: H }, cls = 'shot') {
	const box = el('div', cls, parent);
	const img = el('img', null, box);
	img.src = src;
	img.alt = '';
	img.decoding = 'async';
	const [nw, nh] = dims;
	const c = Math.max(view.w / nw, view.h / nh);
	const bw = nw * c;
	const bh = nh * c;
	css(img, { width: bw + 'px', height: bh + 'px' });
	const tf = (r = [0, 0, 1, 1], zoom = 1) => {
		const [x, y, w, h] = r;
		const k = Math.max(view.w / (w * bw), view.h / (h * bh), 1) * zoom;
		let tx = view.w / 2 - (x + w / 2) * bw * k;
		let ty = view.h / 2 - (y + h / 2) * bh * k;
		tx = Math.min(0, Math.max(view.w - bw * k, tx));
		ty = Math.min(0, Math.max(view.h - bh * k, ty));
		return `translate(${tx.toFixed(1)}px, ${ty.toFixed(1)}px) scale(${k.toFixed(4)})`;
	};
	img.style.transform = tf();
	return { el: box, img, tf };
}

/** Shrink a rect around its centre (a push-in target for Ken Burns). */
export function inset(r, f = 0.9, dx = 0, dy = 0) {
	const [x, y, w, h] = r;
	return [x + (w * (1 - f)) / 2 + dx, y + (h * (1 - f)) / 2 + dy, w * f, h * f];
}

// --- palette (theme.ts) ------------------------------------------------------------------

const ACCENT = { strongShade: -0.22, softAlpha: 0.16, onLight: '#ffffff', onDark: '#0b0c10', threshold: 0.42, inkContrast: 4.5 };

export function parseHex(hex) {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) return null;
	const n = parseInt(m[1], 16);
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
const clamp = (n) => Math.max(0, Math.min(255, Math.round(n)));
const toHex = (...b) => '#' + b.map((v) => clamp(v).toString(16).padStart(2, '0')).join('');
function mix(rgb, amount) {
	const target = amount < 0 ? 0 : 255;
	const t = Math.abs(amount);
	return rgb.map((c) => clamp(c + (target - c) * t));
}
function luminance([r, g, b]) {
	const lin = (c) => {
		const s = c / 255;
		return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}
function contrast(a, b) {
	const [la, lb] = [luminance(a), luminance(b)];
	return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}
function toHsl([r, g, b]) {
	r /= 255;
	g /= 255;
	b /= 255;
	const max = Math.max(r, g, b);
	const min = Math.min(r, g, b);
	const l = (max + min) / 2;
	if (max === min) return [0, 0, l];
	const d = max - min;
	const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
	const h = max === r ? (g - b) / d + (g < b ? 6 : 0) : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
	return [h / 6, s, l];
}
function fromHsl([h, s, l]) {
	const f = (n) => {
		const k = (n + h * 12) % 12;
		const a = s * Math.min(l, 1 - l);
		return clamp(255 * (l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))));
	};
	return [f(0), f(8), f(4)];
}
/**
 * The accent as text on the app's surfaces. theme.ts mixes toward white until the tokens'
 * contrast is met; on broadcast graphics that turns a navy accent (Iron Man) into steel
 * grey, so this walks HSL lightness instead and keeps the colour. Same contrast target.
 */
function ink(rgb) {
	const surface = parseHex('#191c27');
	if (contrast(rgb, surface) >= ACCENT.inkContrast) return rgb;
	const [h, s, l] = toHsl(rgb);
	for (let L = l; L <= 1.0001; L += 0.01) {
		const c = fromHsl([h, s, Math.min(1, L)]);
		if (contrast(c, surface) >= ACCENT.inkContrast) return c;
	}
	return [255, 255, 255];
}

/** The more chromatic of a title's two artwork accents: the one that reads as its colour. */
export function pickAccent(backdrop, poster) {
	const chroma = (h) => {
		const c = parseHex(h);
		return c ? Math.max(...c) - Math.min(...c) : -1;
	};
	return chroma(poster) > chroma(backdrop) ? poster : backdrop;
}

export function palette(accent) {
	const base = parseHex(accent);
	const i = ink(base);
	// fills (rails, chips, rings, glows) need to read on the near-black stage too: an accent
	// darker than that falls back to its ink
	const rgb = contrast(base, parseHex('#07080d')) >= 3 ? base : i;
	return {
		accent: toHex(...rgb),
		base: toHex(...base),
		rgb,
		strong: toHex(...mix(rgb, ACCENT.strongShade)),
		onAccent: luminance(rgb) > ACCENT.threshold ? ACCENT.onDark : ACCENT.onLight,
		ink: toHex(...i),
		inkRgb: i,
		dark: toHex(...mix(base, -0.82))
	};
}

/** Scope a palette to one piece's root, as accentVars() does for a title page. */
export function applyPalette(root, p) {
	root.style.setProperty('--accent', p.accent);
	root.style.setProperty('--accent-rgb', p.rgb.join(' '));
	root.style.setProperty('--accent-strong', p.strong);
	root.style.setProperty('--on-accent', p.onAccent);
	root.style.setProperty('--ink', p.ink);
	root.style.setProperty('--glint-rgb', p.inkRgb.join(' '));
}

// --- misc -------------------------------------------------------------------------------

/** A seeded PRNG so "random" jitter is identical on every render and every seek. */
export function rng(seed = 1) {
	let s = seed >>> 0;
	return () => {
		s = (s + 0x6d2b79f5) >>> 0;
		let t = s;
		t = Math.imul(t ^ (t >>> 15), t | 1);
		t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

export const pad2 = (n) => String(n).padStart(2, '0');

export function addMinutes(hhmm, minutes) {
	const [h, m] = hhmm.split(':').map(Number);
	const total = (h * 60 + m + minutes + 1440) % 1440;
	return `${pad2(Math.floor(total / 60))}:${pad2(total % 60)}`;
}
