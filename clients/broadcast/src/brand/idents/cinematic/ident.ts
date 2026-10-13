// "Cinematic": the Couch TV ident as a short film. The couch at dusk on a hilltop, a slow whip to
// a lit window in town at night, a push through it into the room where the television lights the
// couch, the room falling dark around it; then the approved ending: the popcorn drops onto the
// couch, the two become the Couchverse logo, and couchtv types itself in on the sonic logo.

import { frame } from '../../../core/timeline';
import { timeline, type Ctx, type TL } from '../../../templates/shared';
import { Film } from './film';
import { score } from './score';
import { shot } from './world';

const CSS = `
.cine { position: absolute; inset: 0; overflow: hidden; background: #050505; }
.cine canvas { position: absolute; left: 0; top: 0; width: 1920px; height: 1080px; }
`;

export function cinematic({ root, sound }: Ctx): TL {
	const D = frame(10.52);
	const tl = timeline(root, D);
	const style = document.createElement('style');
	style.textContent = CSS;
	root.appendChild(style);
	const host = document.createElement('div');
	host.className = 'cine';
	root.appendChild(host);
	const canvas = document.createElement('canvas');
	canvas.width = 1920;
	canvas.height = 1080;
	host.appendChild(canvas);
	const film = new Film(canvas, shot);
	// every frame is drawn from t: the value changes every frame, so every frame is captured
	tl.dynamic(
		(t) => Math.round(t * 1000) / 1000,
		(v, wait) => film.frame(Number(v), wait !== null)
	);
	tl.cue('dusk', 0);
	// the whole score is one cue: it builds its own mix (hall, echo, chorus) and schedules
	// every voice from the piece's zero
	tl.sound(0, (when, bus, skip) => score(sound, bus, when - skip, D), D);
	return tl;
}
