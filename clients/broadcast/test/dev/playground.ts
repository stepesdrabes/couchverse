// `npm run dev`: every fixture on a live player, with scrubbing and the matte layer.

import { Player } from '../../src/player';
import type { ClipSpec } from '../../src/spec';
import { FIXTURES, type FixtureName } from '../fixtures/specs';

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const player = new Player($('stage'));
const scrub = $<HTMLInputElement>('scrub');
const part = $<HTMLSelectElement>('part');
const params = new URLSearchParams(location.search);
let name = (params.get('f') as FixtureName) || 'promo-cinematic';

function spec(): ClipSpec {
	const s = FIXTURES[name] as ClipSpec;
	return s.template === 'promo' ? { ...s, part: part.value as 'full' | 'body' | 'end' } : s;
}

async function load(at = 0) {
	await player.load(spec(), at);
	scrub.max = String(player.duration);
	for (const b of $('fixtures').children)
		b.setAttribute('aria-pressed', String((b as HTMLElement).dataset.name === name));
}

player.onUpdate = (s) => {
	$('time').textContent = `${s.time.toFixed(2)} / ${s.duration.toFixed(2)} s`;
	$('cue').textContent = s.cue?.name ?? '';
	$('play').textContent = s.state === 'playing' ? 'Pause' : 'Play';
	if (document.activeElement !== scrub) scrub.value = String(s.time);
};

for (const key of Object.keys(FIXTURES) as FixtureName[]) {
	const b = document.createElement('button');
	b.type = 'button';
	b.textContent = key;
	b.dataset.name = key;
	b.onclick = () => {
		name = key;
		void load();
	};
	$('fixtures').appendChild(b);
}
$('play').onclick = () => (player.snapshot().state === 'playing' ? player.pause() : player.play());
scrub.oninput = () => void player.seek(Number(scrub.value));
part.onchange = () => void load();
$('matte').onclick = () => {
	const on = $('matte').getAttribute('aria-pressed') !== 'true';
	$('matte').setAttribute('aria-pressed', String(on));
	player.stage.setLayer(on ? 'matte' : 'picture');
};
document.addEventListener('keydown', (e) => {
	if (e.key === ' ') {
		e.preventDefault();
		$('play').click();
	} else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
		e.preventDefault();
		void player.step(e.key === 'ArrowRight' ? 1 : -1);
	}
});
void load(Number(params.get('t') ?? 0));
