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
		expect(validate({ template: 'sting', duration: 9 })).toBe(false);
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
		const posters = FIXTURES['ident-full'].posters;
		expect(validate({ ...FIXTURES['ident-full'], posters: [...posters, posters[0]] })).toBe(false);
		expect(validate({ ...FIXTURES['squeeze-key'], handover: 'line' })).toBe(false);
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
		expect(images).toEqual(['/test/media/night-backdrop.jpg', '/test/media/night-poster.jpg']);
		expect(beds).toEqual(['/test/media/bed.ogg']);
		expect(collectUrls(FIXTURES['ident-full']).images).toEqual(FIXTURES['ident-full'].posters);
	});
});
