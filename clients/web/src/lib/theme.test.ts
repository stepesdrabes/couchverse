import { describe, expect, it } from 'vitest';
import { palette } from './theme';

// the same values the core's theme module derives (core/crates/app/src/modules/theme.rs)
describe('palette', () => {
	it('derives the accent ink the core derives', () => {
		expect(palette('#e50914')?.ink).toBe('#ec474f');
		expect(palette('#3a6ea5')?.ink).toBe('#5b87b4');
		expect(palette('#facc15')?.ink).toBe('#facc15');
	});

	it('keeps the strong, soft and on-accent colours', () => {
		expect(palette('#e50914')).toMatchObject({
			strong: '#b30710',
			soft: '#e5091429',
			onAccent: '#ffffff'
		});
	});
});
