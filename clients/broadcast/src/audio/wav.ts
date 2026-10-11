/** Interleave channels into a 32-bit float WAV (format 3), which ffmpeg reads losslessly. */
export function encodeWav(channels: Float32Array[], sampleRate: number): ArrayBuffer {
	const n = channels.length;
	const frames = channels[0]?.length ?? 0;
	const dataBytes = frames * n * 4;
	const buf = new ArrayBuffer(44 + dataBytes);
	const v = new DataView(buf);
	const ascii = (at: number, s: string) => {
		for (let i = 0; i < s.length; i++) v.setUint8(at + i, s.charCodeAt(i));
	};
	ascii(0, 'RIFF');
	v.setUint32(4, 36 + dataBytes, true);
	ascii(8, 'WAVE');
	ascii(12, 'fmt ');
	v.setUint32(16, 16, true);
	v.setUint16(20, 3, true);
	v.setUint16(22, n, true);
	v.setUint32(24, sampleRate, true);
	v.setUint32(28, sampleRate * n * 4, true);
	v.setUint16(32, n * 4, true);
	v.setUint16(34, 32, true);
	ascii(36, 'data');
	v.setUint32(40, dataBytes, true);
	let at = 44;
	for (let i = 0; i < frames; i++)
		for (let c = 0; c < n; c++) {
			v.setFloat32(at, channels[c][i], true);
			at += 4;
		}
	return buf;
}

/** Read a float or 16-bit PCM WAV back into channels (tests and tooling). */
export function decodeWav(buf: ArrayBuffer): { sampleRate: number; channels: Float32Array[] } {
	const v = new DataView(buf);
	const format = v.getUint16(20, true);
	const n = v.getUint16(22, true);
	const sampleRate = v.getUint32(24, true);
	const bits = v.getUint16(34, true);
	const bytes = bits / 8;
	const frames = v.getUint32(40, true) / (n * bytes);
	const channels = Array.from({ length: n }, () => new Float32Array(frames));
	let at = 44;
	for (let i = 0; i < frames; i++)
		for (let c = 0; c < n; c++) {
			channels[c][i] = format === 3 ? v.getFloat32(at, true) : v.getInt16(at, true) / 32768;
			at += bytes;
		}
	return { sampleRate, channels };
}
