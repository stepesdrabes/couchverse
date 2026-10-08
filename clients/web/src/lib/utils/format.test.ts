import { describe, expect, it } from 'vitest';
import { classHeight, qualityLabel } from './format';

describe('qualityLabel', () => {
	it('judges a picture by its class height, not its lines', () => {
		expect(qualityLabel(classHeight(1920, 1080))).toBe('1080P');
		expect(qualityLabel(classHeight(1920, 800))).toBe('1080P');
		expect(qualityLabel(classHeight(1280, 532))).toBe('720P');
		expect(qualityLabel(classHeight(3840, 1600))).toBe('4K');
		expect(qualityLabel(classHeight(1440, 1080))).toBe('1080P');
		expect(qualityLabel(classHeight(720, 576))).toBe('SD');
		expect(qualityLabel(classHeight(0, 0))).toBeNull();
	});
});
