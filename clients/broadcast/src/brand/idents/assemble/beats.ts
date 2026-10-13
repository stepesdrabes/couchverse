// The grid picture and sound share: 93.75 bpm, a beat is 16 frames (0.64 s). The landing is the
// downbeat of bar 3; the two beats before bar 1 are the opening's pickup.

export const B = 0.64;
export const at = (beat: number) => beat * B;

export const T = {
	/** The second eclipse: the white circle in front, on bar 1. */
	eclipse: at(2),
	/** The circles lock as the arms; the white one turns red. */
	armsLock: at(4),
	/** The dome has risen (bar 2). */
	domeLock: at(6),
	/** The slab has glided in under them. */
	baseLock: at(7),
	footL: at(7.25),
	footR: at(7.5),
	/** The knockout gaps close: one glyph. */
	fuse: at(8),
	/** The popcorn starts to fall. */
	fall: at(8.875),
	/** It lands: the logo (bar 3). */
	land: at(10),
	/** couch, t, v. */
	notes: [at(11), at(11.5), at(12)]
};

/** The ident's length: 263 frames. */
export const LENGTH = 10.52;
