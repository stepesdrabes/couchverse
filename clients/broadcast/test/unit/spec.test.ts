import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import Ajv from 'ajv';
import { describe, expect, it } from 'vitest';
import { airing } from '../../src/airing';
import { collectUrls } from '../../src/assets';
import { TEMPLATE_NAMES } from '../../src/templates';
import { BREAK, FIXTURES } from '../fixtures/specs';

const schema = JSON.parse(
	readFileSync(
		resolve(import.meta.dirname, '../../../../contract/broadcast-spec.schema.json'),
		'utf8'
	)
);
const validate = new Ajv({ allErrors: true, strict: false }).compile(schema);

describe('the spec schema', () => {
	it('accepts every fixture and the break', () => {
		for (const [name, spec] of Object.entries(FIXTURES)) {
			expect(validate(spec), `${name}: ${JSON.stringify(validate.errors)}`).toBe(true);
		}
		expect(validate(BREAK), JSON.stringify(validate.errors)).toBe(true);
	});

	it('has a fixture for every template', () => {
		const used = new Set(Object.values(FIXTURES).map((s) => s.template));
		for (const t of TEMPLATE_NAMES) expect(used, t).toContain(t);
	});

	it('rejects what the templates cannot show', () => {
		expect(validate({ template: 'ident', cut: 'medium' })).toBe(false);
		expect(validate({ template: 'sting', duration: 10.04 })).toBe(false);
		expect(validate({ template: 'sting', duration: 4.96 })).toBe(false);
		expect(validate({ template: 'sting', duration: 5 })).toBe(true);
		expect(validate({ template: 'sting', duration: 10 })).toBe(true);
		expect(validate({ ...FIXTURES.upnext, time: '8pm' })).toBe(false);
		expect(
			validate({
				...FIXTURES.upnext,
				title: { ...FIXTURES.upnext.title, accents: { poster: 'red' } }
			})
		).toBe(false);
		expect(validate({ ...FIXTURES['promo-cinematic'], shots: [] })).toBe(false);
		expect(validate({ ...FIXTURES.lineup, items: FIXTURES.lineup.items.slice(0, 1) })).toBe(false);
		expect(validate({ ...FIXTURES.standby, extra: true })).toBe(false);
		expect(validate({ ...FIXTURES['ident-morph'], design: 'frame' })).toBe(false);
		expect(validate({ ...FIXTURES['ident-short'], posters: [] })).toBe(false);
		expect(validate({ ...FIXTURES['squeeze-key'], handover: 'line' })).toBe(false);
		expect(validate({ ...FIXTURES['squeeze-key'], handover: 'cursor' })).toBe(false);
	});
});

describe('airing phrases', () => {
	it('format in English', () => {
		expect(airing('tonight', '20:15')).toEqual({ eyebrow: 'Tonight', time: '20:15' });
		expect(airing('premiere', '20:15', 5)).toEqual({
			eyebrow: 'Premiere',
			day: 'Saturday',
			time: '20:15'
		});
		expect(validate({ ...FIXTURES['promo-cinematic'], airing: airing('weekdays', '18:00') })).toBe(
			true
		);
	});
});

describe('collectUrls', () => {
	it('finds artwork and beds, not colours or footage frames', () => {
		const { images, beds } = collectUrls(FIXTURES['promo-cinematic']);
		expect(images).toEqual([
			'/test/media/night-backdrop.jpg',
			'/test/media/night-poster.jpg',
			'/test/media/logo-wide.png'
		]);
		expect(beds).toEqual(['/test/media/bed.ogg']);
		expect(collectUrls(FIXTURES['ident-journey'])).toEqual({ images: [], beds: [] });
	});

	it('loads a title logo with the artwork, footage frame by frame', () => {
		const title = {
			name: 'Logo',
			genres: [],
			accents: {},
			backdrop: '/b.jpg',
			poster: '/p.jpg',
			logo: { url: '/logo.png', aspect: 4 },
			footage: [{ kind: 'footage' as const, frames: ['/shot/001.jpg'] }]
		};
		expect(collectUrls({ template: 'upNext', title, time: '20:15' }).images).toEqual([
			'/b.jpg',
			'/p.jpg',
			'/logo.png'
		]);
	});

	it('takes an empty URL for no artwork, not one to load', () => {
		const title = { name: 'No Art', genres: [], accents: {}, backdrop: '', poster: '' };
		expect(collectUrls({ template: 'upNext', title, time: '20:15' }).images).toEqual([]);
	});
});
