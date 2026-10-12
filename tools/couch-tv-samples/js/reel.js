// The Couch TV sample reel: sample data (titles, schedule), asset preloading, and the player
// that runs one piece or a whole break against the audio clock.
//
// URL params: ?piece=<id>&t=<seconds>&lang=en|cs&mute=1 freezes a piece at a time;
// variant=a|b and theme=1|2|3 pick the ident and the theme arrangement (the tags follow).

import { AudioEngine, renderMix } from './audio.js';
import * as T from './templates.js';
import { el, rng } from './core.js';
import { loudness, spectrum } from './analyze.js';
import { keyReport, THEME_NAMES } from './theme.js';

// the recommended pairing; the control bar and the URL can pick any other
const DEFAULT_PICKS = { variant: 'b', theme: 1 };

// --- sample data -------------------------------------------------------------------------------

const ART_IDS = ['ironman', 'pulp', 'hp1', 'dexter', 'simpsons', 'invincible', 'southpark'];
const BEDS = {
	ironman: 'assets/audio/ironman.m4a',
	pulpfiction: 'assets/audio/pulpfiction.m4a',
	harrypotter: 'assets/audio/harrypotter.m4a',
	simpsons: 'assets/audio/simpsons.m4a',
	southpark: 'assets/audio/southpark.m4a',
	dexter: 'assets/audio/dexter.m4a',
	invincible: 'assets/audio/invincible.m4a'
};

const G = {
	action: { en: 'Action', cs: 'Akční' },
	adventure: { en: 'Adventure', cs: 'Dobrodružný' },
	scifi: { en: 'Sci-Fi', cs: 'Sci-fi' },
	crime: { en: 'Crime', cs: 'Krimi' },
	thriller: { en: 'Thriller', cs: 'Thriller' },
	comedy: { en: 'Comedy', cs: 'Komedie' },
	animation: { en: 'Animation', cs: 'Animovaný' },
	drama: { en: 'Drama', cs: 'Drama' },
	fantasy: { en: 'Fantasy', cs: 'Fantasy' }
};

// Accents are the server-extracted artwork accents. Shot rects are normalized crops of the
// backdrop (b) or poster (p): in the product they would come from saliency/face detection
// or an admin's picks, with a default set when nothing is known.
const TITLES = {
	ironman: {
		name: 'Iron Man',
		year: 2008,
		runtime: 126,
		genres: [G.action, G.adventure, G.scifi],
		accents: { backdrop: '#0a2a44', poster: '#072646' },
		art: 'ironman',
		bed: 'ironman',
		tagline: { en: ['Heroes aren’t born.', 'They’re built.'], cs: ['Hrdinové se nerodí.', 'Vyrábějí se.'] },
		shots: [
			{ src: 'b', from: [0, 0, 1, 1], to: [0.4, 0.02, 0.46, 0.46] },
			{ src: 'p', from: [0.0, 0.03, 0.62, 0.26], to: [0.06, 0.06, 0.5, 0.2] },
			{ src: 'b', from: [0.24, 0.05, 0.22, 0.42], to: [0.27, 0.08, 0.17, 0.32] },
			{ src: 'p', from: [0.0, 0.47, 0.46, 0.2], to: [0.02, 0.5, 0.38, 0.16] },
			{ src: 'b', from: [0.52, 0.5, 0.4, 0.48], to: [0.56, 0.55, 0.32, 0.4] },
			{ src: 'b', from: [0.37, 0.14, 0.26, 0.44], to: [0.4, 0.18, 0.2, 0.36] },
			{ src: 'p', from: [0.56, 0.0, 0.44, 0.24], to: [0.6, 0.02, 0.36, 0.2] },
			{ src: 'b', from: [0.66, 0.4, 0.26, 0.46], to: [0.69, 0.44, 0.2, 0.38] }
		],
		hero: { src: 'b', from: [0.4, 0, 0.5, 0.7], to: [0.47, 0.03, 0.38, 0.55] },
		frames: { lineup: [0.25, 0, 0.62, 0.9] }
	},
	pulp: {
		name: 'Pulp Fiction',
		year: 1994,
		runtime: 154,
		genres: [G.crime, G.thriller],
		accents: { backdrop: '#643906', poster: '#d62316' },
		art: 'pulp',
		bed: 'pulpfiction',
		tagline: { en: ['Uninterrupted.', 'As it should be.'], cs: ['Bez přerušení.', 'Jak se patří.'] },
		shots: [
			{ src: 'b', from: [0.0, 0.22, 0.36, 0.34] },
			{ src: 'b', from: [0.58, 0.0, 0.24, 0.42] },
			{ src: 'b', from: [0.33, 0.06, 0.2, 0.36] },
			{ src: 'p', from: [0.1, 0.32, 0.46, 0.22] },
			{ src: 'b', from: [0.34, 0.33, 0.24, 0.22] },
			{ src: 'p', from: [0.18, 0.7, 0.44, 0.17] },
			{ src: 'p', from: [0.0, 0.4, 0.34, 0.17] },
			{ src: 'b', from: [0, 0, 1, 1] },
			{ src: 'p', from: [0.45, 0.17, 0.5, 0.2] },
			{ src: 'p', from: [0.36, 0.8, 0.44, 0.18] }
		],
		hero: { src: 'b', from: [0, 0, 1, 1], to: [0.05, 0.04, 0.9, 0.9] },
		frames: {}
	},
	simpsons: {
		name: { en: 'The Simpsons', cs: 'Simpsonovi' },
		genres: [G.comedy, G.animation],
		accents: { backdrop: '#f5cc36', poster: '#fed51c' },
		art: 'simpsons',
		bed: 'simpsons',
		frames: {
			upnext: [[0, 0, 1, 1], [0.08, 0.04, 0.88, 0.88]],
			strip: [[0.3, 0, 0.7, 1], [0.36, 0.05, 0.6, 0.86]],
			lineup: [0.3, 0, 0.7, 1]
		}
	},
	southpark: {
		name: 'South Park',
		genres: [G.comedy, G.animation],
		accents: { backdrop: '#7a5a3a', poster: '#a6795b' },
		art: 'southpark',
		bed: 'southpark',
		frames: { lineup: [0.22, 0.12, 0.6, 0.78] }
	},
	dexter: {
		name: 'Dexter',
		genres: [G.crime, G.drama],
		accents: { backdrop: '#d69a77', poster: '#d81d24' },
		art: 'dexter',
		bed: 'dexter',
		frames: {
			strip: [[0.4, 0, 0.6, 1], [0.46, 0.06, 0.5, 0.84]],
			lineup: [0.4, 0, 0.6, 1],
			overlays: [[0, 0, 1, 1], [0.03, 0.02, 0.94, 0.94], [0.36, 0.04, 0.62, 0.8], [0.4, 0.07, 0.56, 0.72]]
		}
	},
	invincible: {
		name: 'Invincible',
		genres: [G.animation, G.action],
		accents: { backdrop: '#1a2554', poster: '#f8eb17' },
		art: 'invincible',
		bed: 'invincible',
		frames: { short: [[0.18, 0.02, 0.66, 0.86], [0.26, 0.08, 0.52, 0.66]] }
	},
	hp1: {
		name: { en: "Harry Potter and the Philosopher's Stone", cs: 'Harry Potter a Kámen mudrců' },
		genres: [G.fantasy, G.adventure],
		accents: { backdrop: '#771607', poster: '#57b7e6' },
		art: 'hp1',
		bed: 'harrypotter',
		frames: {}
	}
};

const EP = {
	simpsons: { code: 'S01E02', name: { en: 'Bart the Genius', cs: 'Malý génius' }, runtime: 23 },
	dexter: { code: 'S01E02', name: { en: 'Crocodile', cs: 'Krokodýl' }, runtime: 55 }
};

// stand-in crew for the credits roll (the "video" of the ending programme)
const CREDITS = [
	['Directed by', 'Mara Lindqvist'],
	['Screenplay by', 'Tomas Reyes'],
	['Director of Photography', 'Ines Kowalczyk'],
	['Edited by', 'Jonah Whitfield'],
	['Production Designer', 'Adaeze Okafor'],
	['Music by', 'Lukas Hartmann'],
	['Costume Designer', 'Sofia Marchetti'],
	['Casting by', 'Priya Raman'],
	['Visual Effects Supervisor', 'Henrik Dahl'],
	['Sound Designer', 'Camille Laurent'],
	['Re-recording Mixer', 'Oskar Novak'],
	['First Assistant Director', 'Ruth Abernathy'],
	['Supervising Art Director', 'Diego Fuentes'],
	['Script Supervisor', 'Hana Sato'],
	['Gaffer', 'Peter Brandt'],
	['Key Grip', 'Elena Petrova']
];

const PIECES = [
	{ id: 'ident', n: '1', label: { en: 'Ident, full', cs: 'Ident, plný' }, tpl: T.ident, data: () => ({ cut: 'full', ...picks, art: ART }) },
	{ id: 'ident-short', n: '1b', label: { en: 'Ident, short', cs: 'Ident, krátký' }, tpl: T.ident, data: () => ({ cut: 'short', ...picks, art: ART }) },
	{
		id: 'upnext',
		n: '2',
		label: { en: 'Up next', cs: 'Následuje' },
		tpl: T.upNext,
		data: () => ({ title: TITLES.simpsons, episode: EP.simpsons, time: '18:00', clock: '17:59' })
	},
	{
		id: 'ironman',
		n: '3',
		label: { en: 'Tonight promo', cs: 'Upoutávka: dnes' },
		tpl: T.moviePromo,
		data: () => ({ title: TITLES.ironman, mood: 'cinematic', when: { kind: 'tonight', time: '20:15' } })
	},
	{
		id: 'pulp',
		n: '4',
		label: { en: 'Premiere promo', cs: 'Upoutávka: premiéra' },
		tpl: T.moviePromo,
		data: () => ({ title: TITLES.pulp, mood: 'grit', when: { kind: 'premiere', day: 5, time: '20:15' } })
	},
	{
		id: 'simpsons',
		n: '5',
		label: { en: 'Strip promo', cs: 'Pásmo' },
		tpl: T.stripPromo,
		data: () => ({ title: TITLES.simpsons, when: { kind: 'weekdays', days: [0, 1, 2, 3, 4], time: '18:00' }, count: 801 })
	},
	{
		id: 'dexter',
		n: '5b',
		label: { en: 'Strip promo, dark', cs: 'Pásmo, temné' },
		tpl: T.stripPromo,
		data: () => ({
			title: TITLES.dexter,
			mood: 'dark',
			when: { kind: 'nightly', days: [0, 1, 2, 3, 4, 5, 6], time: '22:30', from: { season: 1, day: 0 } }
		})
	},
	{
		id: 'invincible',
		n: '6',
		label: { en: 'Short promo', cs: 'Krátká upoutávka' },
		tpl: T.shortPromo,
		data: () => ({ title: TITLES.invincible, when: { kind: 'tomorrow', time: '21:00' } })
	},
	{
		id: 'lineup',
		n: '7',
		label: { en: 'Lineup', cs: 'Program' },
		tpl: T.lineup,
		data: () => ({
			now: '18:04',
			items: [
				{ time: '18:00', title: TITLES.simpsons, sub: { en: 'S01E02 · Bart the Genius', cs: 'S01E02 · Malý génius' } },
				{ time: '18:25', title: TITLES.southpark, sub: 'S01E02 · Weight Gain 4000' },
				{ time: '20:15', title: TITLES.ironman, sub: { en: 'Movie · Action · 126 min', cs: 'Film · Akční · 126 min' } },
				{ time: '22:30', title: TITLES.dexter, sub: { en: 'S01E02 · Crocodile', cs: 'S01E02 · Krokodýl' } }
			]
		})
	},
	{
		id: 'overlays',
		n: '8',
		label: { en: 'Overlays', cs: 'Překryvy' },
		tpl: T.overlays,
		data: () => ({
			title: TITLES.dexter,
			episode: EP.dexter,
			slot: ['22:30', '23:25'],
			start: '22:30',
			jumpMinutes: 52,
			next: { title: TITLES.southpark, time: '23:30', sub: 'S01E03 · Volcano' }
		})
	},
	{
		id: 'squeeze',
		n: '9',
		label: { en: 'Credits squeeze', cs: 'Zúžení titulků' },
		tpl: T.creditsSqueeze,
		data: () => ({
			ending: TITLES.hp1,
			next: { title: TITLES.simpsons, episode: EP.simpsons, time: '18:00' },
			clock: '17:57',
			credits: CREDITS,
			bedFrom: 2,
			variant: picks.variant
		})
	}
];

// a real break: end credits, ident, promos, lineup, then the next programme's bumper
const BREAK = [
	{ id: 'squeeze', patch: { handover: 'cursor' } },
	{ id: 'ident-short', overlap: 0 },
	{ id: 'ironman', overlap: 0.35 },
	{ id: 'invincible', overlap: 0.3 },
	{ id: 'lineup', patch: { now: '17:58' }, overlap: 0.35 },
	{ id: 'upnext', overlap: 0.5 }
];

// --- assets ----------------------------------------------------------------------------------

const ART = {};

async function loadImage(src) {
	const img = new Image();
	img.src = src;
	await img.decode();
	return img;
}

/** A static pre-blurred copy (blurred once on a canvas, never animated). */
function preBlur(img) {
	const c = document.createElement('canvas');
	c.width = 384;
	c.height = 216;
	const g = c.getContext('2d');
	if ('filter' in g) g.filter = 'blur(9px)';
	g.drawImage(img, -30, -18, 444, 252);
	return c.toDataURL('image/jpeg', 0.86);
}

function grainTexture() {
	const c = document.createElement('canvas');
	c.width = c.height = 256;
	const g = c.getContext('2d');
	const d = g.createImageData(256, 256);
	const r = rng(99);
	for (let i = 0; i < d.data.length; i += 4) {
		const v = Math.round(r() * 255);
		d.data[i] = d.data[i + 1] = d.data[i + 2] = v;
		d.data[i + 3] = 255;
	}
	g.putImageData(d, 0, 0);
	return c.toDataURL('image/png');
}

async function preload(engine) {
	const jobs = [];
	for (const id of ART_IDS) {
		jobs.push(
			(async () => {
				const [b, p] = await Promise.all([loadImage(`assets/art/${id}/backdrop.jpg`), loadImage(`assets/art/${id}/poster.jpg`)]);
				ART[id] = {
					b: b.src,
					p: p.src,
					blur: preBlur(b),
					dims: { b: [b.naturalWidth, b.naturalHeight], p: [p.naturalWidth, p.naturalHeight] },
					_keep: [b, p]
				};
			})()
		);
	}
	jobs.push(engine.load(BEDS));
	jobs.push(
		Promise.all([
			document.fonts.load('700 100px Saira', 'C()UCH TV 20:15 ĎŽÁČŠŘ'),
			document.fonts.load('800 100px Jakarta', 'Couch TV ČŠŘŽ'),
			document.fonts.load('700 100px Jakarta', 'Aa ěščřžýáíéůú'),
			document.fonts.load('600 100px Jakarta', 'Aa ěščřžýáíéůú'),
			document.fonts.load('500 100px Jakarta', 'Aa'),
			document.fonts.load('400 100px Jakarta', 'Aa')
		])
	);
	await Promise.all(jobs);
	document.documentElement.style.setProperty('--grain', `url(${grainTexture()})`);
	for (const t of Object.values(TITLES)) t.art = typeof t.art === 'string' ? ART[t.art] : t.art;
}

/** Resolve every {en, cs} leaf to one language (a deep copy, so templates may mutate it). */
function localize(v, lang) {
	if (Array.isArray(v)) return v.map((x) => localize(x, lang));
	if (v && typeof v === 'object') {
		const keys = Object.keys(v);
		if (keys.length === 2 && 'en' in v && 'cs' in v) return localize(v[lang], lang);
		if (v instanceof HTMLElement) return v;
		const o = {};
		for (const k of keys) o[k] = k === '_keep' ? v[k] : localize(v[k], lang);
		return o;
	}
	return v;
}

// --- player ----------------------------------------------------------------------------------

const engine = new AudioEngine();
const $ = (id) => document.getElementById(id);
const viewport = $('viewport');
const stage = $('stage');
const scrub = $('scrub');
const params = new URLSearchParams(location.search);

let lang = params.get('lang') === 'cs' ? 'cs' : 'en';
const picks = {
	variant: ['a', 'b'].includes(params.get('variant')) ? params.get('variant') : DEFAULT_PICKS.variant,
	theme: [1, 2, 3].includes(+params.get('theme')) ? +params.get('theme') : DEFAULT_PICKS.theme
};
engine.theme = picks.theme;
let state = { mode: 'idle', pieces: [] };

function fitStage() {
	const vw = viewport.clientWidth;
	const vh = viewport.clientHeight;
	const k = Math.min(vw / 1920, vh / 1080);
	stage.style.transform = `translate(${(vw - 1920 * k) / 2}px, ${(vh - 1080 * k) / 2}px) scale(${k})`;
}
new ResizeObserver(fitStage).observe(viewport);

function build(id, patch = {}, below = null, audio = engine) {
	const p = PIECES.find((x) => x.id === id);
	const layer = el('div', 'piece');
	layer.dataset.piece = id;
	if (below) stage.insertBefore(layer, below);
	else stage.appendChild(layer);
	const data = localize({ ...p.data(), ...patch }, lang);
	data.lang = lang;
	const tl = p.tpl(layer, data, audio);
	return { id, p, layer, tl, bus: null };
}

function dispose(piece) {
	piece.bus?.stop(0.08);
	piece.tl.cancel();
	piece.layer.remove();
}

function clearAll() {
	for (const p of state.pieces) dispose(p);
	engine.stopAll();
	state = { mode: 'idle', pieces: [] };
}

async function unlockAudio() {
	try {
		await engine.unlock();
	} catch (err) {
		console.warn('audio unavailable', err);
	}
}

function launch(piece, docStart, ctxStart, offset = 0) {
	piece.tl.play(docStart, offset);
	if (engine.running && ctxStart != null) {
		piece.bus = engine.bus();
		piece.tl.scheduleSounds(ctxStart, offset, piece.bus);
	}
}

function clocks() {
	return engine.running ? engine.clockPair(0.12) : { docStart: document.timeline.currentTime + 60, ctxStart: null };
}

async function playSingle(id, from = 0) {
	await unlockAudio();
	clearAll();
	const piece = build(id);
	const { docStart, ctxStart } = clocks();
	launch(piece, docStart, ctxStart, from);
	state = { mode: 'single', pieces: [piece], cur: piece };
	syncUI();
}

function freeze(id, t, keep = false, patch = {}) {
	let piece = keep && state.cur && state.cur.id === id ? state.cur : null;
	if (!piece) {
		clearAll();
		piece = build(id, patch);
	} else {
		piece.bus?.stop(0.06);
		piece.bus = null;
	}
	t = Math.max(0, Math.min(piece.tl.duration, t));
	piece.tl.seek(t);
	state = { mode: 'frozen', pieces: [piece], cur: piece, t };
	syncUI();
}

async function resume() {
	const piece = state.cur;
	if (!piece) return playSingle(currentId());
	await unlockAudio();
	let t = state.t ?? 0;
	if (t >= piece.tl.duration - 0.05) t = 0;
	piece.bus?.stop(0.1);
	const { docStart, ctxStart } = clocks();
	launch(piece, docStart, ctxStart, t);
	state = { mode: 'single', pieces: [piece], cur: piece };
	syncUI();
}

function pause() {
	if (state.mode === 'break') {
		const t = state.cur.tl.time();
		const cur = state.cur;
		for (const p of state.pieces) if (p !== cur) dispose(p);
		state = { mode: 'single', pieces: [cur], cur };
		return freeze(cur.id, t, true);
	}
	if (state.mode !== 'single') return;
	freeze(state.cur.id, state.cur.tl.time(), true);
}

async function playBreak() {
	await unlockAudio();
	clearAll();
	const first = build(BREAK[0].id, BREAK[0].patch);
	const { docStart, ctxStart } = clocks();
	launch(first, docStart, ctxStart);
	first.startDoc = docStart;
	const total = BREAK.reduce((s, b, i) => s, 0);
	state = { mode: 'break', pieces: [first], cur: first, idx: 0, doc0: docStart, ctx0: ctxStart, total };
	state.nextStart = docStart + (first.tl.duration - (BREAK[1].overlap || 0)) * 1000;
	syncUI();
}

function breakStep(now) {
	const s = state;
	const nextDef = BREAK[s.idx + 1];
	if (nextDef && !s.next && now >= s.nextStart - 1400) {
		// build the incoming piece under the current one, a little ahead of time
		const piece = build(nextDef.id, nextDef.patch, s.cur.layer);
		const ctxStart = s.ctx0 != null ? s.ctx0 + (s.nextStart - s.doc0) / 1000 : null;
		launch(piece, s.nextStart, ctxStart);
		piece.startDoc = s.nextStart;
		s.pieces.push(piece);
		s.next = piece;
		const overlap = nextDef.overlap || 0;
		const out = s.cur;
		const fade = out.layer.animate([{ opacity: 1 }, { opacity: 0 }], { duration: Math.max(1, overlap * 1000), fill: 'forwards', easing: 'cubic-bezier(0.37, 0, 0.63, 1)' });
		fade.startTime = s.nextStart;
		if (out.bus && ctxStart != null) out.bus.fade(0, ctxStart, Math.max(0.05, overlap));
		out.retireAt = s.nextStart + overlap * 1000 + 250;
	}
	if (s.next && now >= s.nextStart) {
		s.cur = s.next;
		s.next = null;
		s.idx++;
		const after = BREAK[s.idx + 1];
		if (after) s.nextStart = s.cur.startDoc + (s.cur.tl.duration - (after.overlap || 0)) * 1000;
		syncUI();
	}
	for (const p of [...s.pieces]) {
		if (p.retireAt && now >= p.retireAt) {
			dispose(p);
			s.pieces.splice(s.pieces.indexOf(p), 1);
		}
	}
}

function frame() {
	const now = document.timeline.currentTime;
	if (state.mode === 'break') breakStep(now);
	if (state.mode === 'single' || state.mode === 'break') {
		for (const p of state.pieces) {
			const t = p.tl.time(now);
			if (t != null) p.tl.update(Math.max(0, t));
		}
		const cur = state.cur;
		const t = cur.tl.time(now);
		if (t >= cur.tl.duration && (state.mode === 'single' || !BREAK[state.idx + 1])) {
			// natural end: hold the last frame and let reverb tails ring out
			for (const p of state.pieces) if (p !== cur) dispose(p);
			cur.tl.seek(cur.tl.duration);
			state = { mode: 'frozen', pieces: [cur], cur, t: cur.tl.duration };
			syncUI();
		} else readout(Math.max(0, t));
	}
	requestAnimationFrame(frame);
}

// --- UI ----------------------------------------------------------------------------------------

function currentId() {
	return state.cur?.id || 'ident';
}

function readout(t) {
	const cur = state.cur;
	if (!cur) return;
	$('tRead').textContent = t.toFixed(2);
	if (document.activeElement !== scrub) scrub.value = t;
	const c = cur.tl.cueAt(t);
	const lbl = cur.p.label[lang] + (cur.id.startsWith('ident') ? ` ${picks.variant.toUpperCase()}${picks.theme}` : '');
	$('cueRead').innerHTML = '';
	el('b', null, $('cueRead'), `${cur.p.n} ${lbl}`);
	if (c) $('cueRead').append(` · ${c.name}`);
}

function syncUI() {
	const cur = state.cur;
	const playing = state.mode === 'single' || state.mode === 'break';
	$('playBtn').classList.toggle('playing', playing);
	$('playBtn').setAttribute('aria-label', playing ? 'Pause' : 'Play');
	$('breakBtn').setAttribute('aria-pressed', String(state.mode === 'break'));
	$('breakBtn').textContent = lang === 'cs' ? 'Přehrát celý blok' : 'Play full break';
	for (const b of $('pieces').children) {
		b.setAttribute('aria-selected', String(cur && b.dataset.id === cur.id));
		const p = PIECES.find((x) => x.id === b.dataset.id);
		b.lastChild.textContent = p.label[lang];
	}
	for (const b of $('langSeg').children) b.setAttribute('aria-pressed', String(b.dataset.lang === lang));
	for (const b of $('variantSeg').children) b.setAttribute('aria-pressed', String(b.dataset.variant === picks.variant));
	for (const b of $('themeSeg').children) {
		b.setAttribute('aria-pressed', String(+b.dataset.theme === picks.theme));
		b.title = THEME_NAMES[b.dataset.theme][lang];
	}
	$('identLabel').textContent = lang === 'cs' ? 'Ident' : 'Ident';
	$('themeLabel').textContent = lang === 'cs' ? 'Znělka' : 'Theme';
	$('muteBtn').classList.toggle('muted', engine.muted);
	if (!cur) return;
	scrub.max = cur.tl.duration;
	$('dRead').textContent = cur.tl.duration.toFixed(2);
	const marks = $('cueMarks');
	marks.innerHTML = '';
	for (const c of cur.tl.cues) {
		const m = el('i', null, marks);
		m.style.left = `calc(${(c.at / cur.tl.duration) * 100}% - 1px)`;
		m.title = `${c.name} @ ${c.at.toFixed(2)} s`;
	}
	readout(state.mode === 'frozen' ? state.t : Math.max(0, cur.tl.time() ?? 0));
}

function buildControls() {
	for (const p of PIECES) {
		const b = el('button', 'piece-btn', $('pieces'));
		b.type = 'button';
		b.setAttribute('role', 'tab');
		b.dataset.id = p.id;
		el('b', null, b, p.n);
		el('span', null, b, p.label[lang]);
		b.addEventListener('click', () => playSingle(p.id, 0));
	}
	$('breakBtn').addEventListener('click', () => (state.mode === 'break' ? pause() : playBreak()));
	$('playBtn').addEventListener('click', () => {
		if (state.mode === 'single' || state.mode === 'break') pause();
		else resume();
	});
	scrub.addEventListener('input', () => {
		const id = currentId();
		if (state.mode === 'break') pause();
		freeze(id, +scrub.value, true);
	});
	for (const b of $('langSeg').children) {
		b.addEventListener('click', () => setLang(b.dataset.lang));
	}
	for (const b of $('variantSeg').children) b.addEventListener('click', () => setPick({ variant: b.dataset.variant }));
	for (const b of $('themeSeg').children) b.addEventListener('click', () => setPick({ theme: +b.dataset.theme }));
	$('muteBtn').addEventListener('click', async () => {
		engine.setMuted(!engine.muted);
		if (!engine.muted) await unlockAudio();
		syncUI();
	});
	document.addEventListener('keydown', (e) => {
		if (e.target instanceof HTMLInputElement && e.key !== ' ') return;
		if (e.key === ' ') {
			e.preventDefault();
			$('playBtn').click();
		} else if ((e.key === 'ArrowRight' || e.key === 'ArrowLeft') && state.mode === 'frozen') {
			const d = (e.key === 'ArrowRight' ? 1 : -1) * (e.shiftKey ? 1 : 1 / 25);
			freeze(currentId(), state.t + d, true);
		}
	});
	// the first gesture anywhere wakes the audio context
	document.addEventListener('pointerdown', () => unlockAudio(), { once: true });
}

/** A new ident variant or theme rebuilds what is on screen (and is kept in the URL). */
async function setPick(p) {
	Object.assign(picks, p);
	engine.theme = picks.theme;
	const u = new URL(location.href);
	u.searchParams.set('variant', picks.variant);
	u.searchParams.set('theme', String(picks.theme));
	history.replaceState(null, '', u);
	await rebuild();
}

async function setLang(l) {
	if (l === lang) return;
	lang = l;
	document.documentElement.lang = l;
	await rebuild();
}

async function rebuild() {
	const id = currentId();
	if (state.mode === 'break') {
		syncUI();
		return;
	}
	const t = state.mode === 'single' ? state.cur.tl.time() : (state.t ?? 0);
	if (state.mode === 'single') {
		await playSingle(id, t);
	} else freeze(id, t);
}

async function main() {
	if (params.get('still') === '1') document.body.classList.add('still');
	buildControls();
	if (params.get('mute') === '1') engine.setMuted(true);
	document.documentElement.lang = lang;
	await preload(engine);
	$('loading').hidden = true;
	const id = PIECES.some((p) => p.id === params.get('piece')) ? params.get('piece') : 'ident';
	const t = params.has('t') ? parseFloat(params.get('t')) : id === 'ident' && !params.has('piece') ? 8.9 : 0;
	freeze(id, t);
	requestAnimationFrame(frame);
	window.reel = {
		freeze,
		playSingle,
		playBreak,
		engine,
		PIECES,
		get state() {
			return state;
		},
		// debug: offline render of one piece's audio (full mix, or solo 'music' / 'sfx')
		async mixReport(id, solo = null, raw = false) {
			let piece;
			const { stats, buffer } = await renderMix(engine, (probe) => (piece = build(id, {}, null, probe)).tl, { solo, raw });
			dispose(piece);
			this.lastBuffer = buffer;
			// a cheap fingerprint, to prove two offline renders are identical
			const ch = buffer.getChannelData(0);
			let h = 0;
			for (let i = 0; i < ch.length; i += 97) h = (h * 31 + Math.round(ch[i] * 1e6)) | 0;
			return { ...stats, hash: h };
		},
		/**
		 * debug: the mix metered like a broadcast deliverable. `patch` is piece data
		 * (variant, theme); raw skips the dynamics to read the headroom going into them.
		 */
		async analyze(id, patch = {}, { solo = null, raw = false, from = 0 } = {}) {
			let piece;
			const keep = engine.theme;
			if (patch.theme) engine.theme = patch.theme;
			const { buffer } = await renderMix(engine, (probe) => (piece = build(id, patch, null, probe)).tl, { solo, raw });
			engine.theme = keep;
			dispose(piece);
			this.lastBuffer = buffer;
			const ch = buffer.getChannelData(0);
			let h = 0;
			for (let i = 0; i < ch.length; i += 97) h = (h * 31 + Math.round(ch[i] * 1e6)) | 0;
			return { ...loudness(buffer), bands: spectrum(buffer, from), hash: h };
		},
		keyReport
	};
}

main().catch((err) => {
	console.error(err);
	$('loading').textContent = 'Could not load the reel: ' + err.message;
});
