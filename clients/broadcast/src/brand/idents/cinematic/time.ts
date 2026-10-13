// The ident's clock. Picture and score share one grid: a 0.6 s beat (100 bpm, 15 frames).

export const BEAT = 0.6;
export const at = (beat: number) => Math.round(beat * BEAT * 1000) / 1000;

export const D = 10.52;

export const T = {
	/** Out of black. */
	fadeIn: 0.6,
	/** The whip from the dusk hill to the town at night. */
	panFrom: 1.5,
	panTo: 2.75,
	/** The push into the window and on through the room to the couch. */
	dollyFrom: 2.0,
	dollyPeak: at(7),
	dollyTo: at(11),
	/** The room falls dark around the couch. */
	darkFrom: 4.6,
	darkTo: 5.8,
	/** The popcorn drops on beat 10 and lands on beat 11. */
	fall: at(10),
	land: at(11),
	/** couch, t, v: the sonic logo. */
	notes: [at(12), at(12.5), at(13)]
};

export const RED = '#e50914';
export const BLACK = '#050505';
export const WHITE = '#ffffff';
