// The picture: one camera travelling through three lit scenes and into the lockup.
//
//   A  dusk: the couch on a hilltop, the sun setting behind it, hills in haze.
//   -> a slow whip east, the sky darkening to night as the camera turns
//   B  night: a building at the edge of town, one window lit, the couch inside.
//   -> a push through the window
//   C  the room: the couch lit by the television, its shadow up the wall. The room falls dark
//      until the couch is alone on black, where the popcorn lands and the logo forms.

import { type Cam, layer, project, rgba, smooth } from './art';
import { camera, LOCK, REF_A, REF_B, ROOM } from './camera';
import { drawClouds, drawDuskFar, drawDuskNear, drawSky, drawSun } from './dusk';
import { couchPress, drawPopcornAt, drawWord, endingMotion, frameGive } from './ending';
import { H, W, type Shot } from './film';
import {
	couchMatrix,
	drawFacade,
	drawMoon,
	drawRoom,
	drawRoomCouch,
	drawSkyline,
	outside,
	roomLight
} from './night';
import { BLACK, T } from './time';

/** The couch where the lockup has it, pressed down a hair by the landing. */
function lockMatrix(t: number) {
	return new DOMMatrix([LOCK.k, 0, 0, LOCK.k, LOCK.x, LOCK.y + couchPress(t) + frameGive(t)]);
}

export const shot: Shot = {
	draw(ctx, t) {
		const cam = camera(t);
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.globalAlpha = 1;
		ctx.fillStyle = BLACK;
		ctx.fillRect(0, 0, W, H);
		if (outside(cam)) {
			drawSky(ctx, cam);
			if (cam.pan < 2300) drawSun(ctx, cam, t);
			if (cam.pan < 2400) drawClouds(ctx, cam, t);
			if (cam.pan > 600) drawMoon(ctx, cam);
			drawDuskFar(ctx, cam);
			if (cam.pan > 300) drawSkyline(ctx, cam);
			drawDuskNear(ctx, cam);
			if (cam.pan > 500) drawFacade(ctx, cam, t);
		} else {
			const lit = roomLight(t);
			if (lit > 0) {
				drawRoom(ctx, cam, t);
				if (lit < 1) {
					ctx.setTransform(1, 0, 0, 1, 0, 0);
					ctx.fillStyle = rgba(BLACK, 1 - lit);
					ctx.fillRect(0, 0, W, H);
				}
			}
			const m = t >= T.dollyTo ? lockMatrix(t) : couchMatrix(cam);
			if (m) {
				drawRoomCouch(ctx, cam, t, m);
				drawPopcornAt(ctx, t, m);
			}
		}
		drawWord(ctx, t);
		const fade = 1 - smooth(0, T.fadeIn, t);
		if (fade > 0) {
			ctx.setTransform(1, 0, 0, 1, 0, 0);
			ctx.globalAlpha = 1;
			ctx.fillStyle = rgba(BLACK, fade);
			ctx.fillRect(0, 0, W, H);
		}
	},
	motion(t) {
		const dt = 1 / 25;
		const a = camera(t);
		const b = camera(t + dt);
		let worst = Math.abs(b.pan - a.pan) + Math.abs(b.tilt - a.tilt);
		worst = Math.max(worst, layersMotion(a, b), roomMotion(a, b), endingMotion(t));
		return worst;
	}
};

/** The fastest screen corner among the dusk layers. */
function layersMotion(a: Cam, b: Cam) {
	let worst = 0;
	for (const d of [380, 1000, 1800]) {
		const ma = layer(a, REF_A, d);
		const mb = layer(b, REF_A, d);
		if (!ma || !mb) continue;
		const inv = ma.inverse();
		for (const [x, y] of [
			[0, 0],
			[W, 0],
			[0, H],
			[W, H]
		]) {
			const q = mb.transformPoint(inv.transformPoint(new DOMPoint(x, y)));
			worst = Math.max(worst, Math.hypot(q.x - x, q.y - y));
		}
	}
	return worst;
}

const PROBES: [number, number, number][] = (() => {
	const r = ROOM;
	const out: [number, number, number][] = [];
	const xs = [r.x0, r.win.x0, 0, r.win.x1, r.x1];
	const ys = [r.ceil, r.win.y0, 0, r.win.y1, r.floor];
	for (const z of [r.wallZ, r.wallIn, 560, 640, 720, 800, 900, r.couchZ, r.back])
		for (const x of xs) for (const y of ys) out.push([x, y, z]);
	return out;
})();

/** The fastest probe of the town and the room that is on (or near) the screen. */
function roomMotion(a: Cam, b: Cam) {
	let worst = 0;
	const dz = a.z - REF_B.z;
	for (const [x, y, z] of PROBES) {
		if (z - dz < 12) continue;
		const p = project(a, REF_B, x, y, z);
		if (p[0] < -400 || p[0] > W + 400 || p[1] < -400 || p[1] > H + 400) continue;
		const q = project(b, REF_B, x, y, z);
		worst = Math.max(worst, Math.hypot(q[0] - p[0], q[1] - p[1]));
	}
	return worst;
}
