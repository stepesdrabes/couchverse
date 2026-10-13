// "Morph": the Couch TV ident in one continuous move. The picture (scene.ts) is a canvas
// redrawn from t; the music (score.ts) is one cue scheduled at the start of the timeline.

import { css, div } from '../../../core/dom';
import { frame } from '../../../core/timeline';
import { timeline, type Ctx, type TL } from '../../../templates/shared';
import { BLACK, Picture, T } from './scene';
import { score } from './score';

export function morph({ root, sound }: Ctx): TL {
	const D = frame(10.52);
	const tl = timeline(root, D);
	const box = css(div(null, root), {
		position: 'absolute',
		inset: '0',
		overflow: 'hidden',
		background: BLACK
	});
	const pic = new Picture(box);
	// the frame's time while anything moves; one value for the still end, so the renderer
	// reuses that frame
	tl.dynamic(
		(t) => (t >= T.still ? 'still' : t),
		(v, wait) => pic.render(typeof v === 'number' ? v : T.still, wait !== null)
	);
	tl.cue('night', 0);
	tl.cue('the moon sets into the sun', T.m1[0]);
	tl.cue('the light recedes: a planet', T.m2[0]);
	tl.cue('the planet lands: the stage', T.m3[0]);
	tl.cue('the popcorn falls', T.drop);
	tl.cue('it lands: the logo', T.land);
	tl.cue('couchtv', T.notes[0]);
	tl.sound(0, (when, bus, skip) => score(sound, bus, when - skip, D), D);
	return tl;
}
