import { describe, expect, it } from 'vitest';
import { bezierPoints, cubicBezierAt, EASE, spring, SPRING } from '../../src/core/motion';
import { curves } from '../../src/generated/tokens';

describe('curves', () => {
	it('come from the design tokens', () => {
		expect(EASE.enter).toBe(`cubic-bezier(${curves.enter.bezier.join(', ')})`);
		expect(EASE.standard).toBe(`cubic-bezier(${curves.standard.bezier.join(', ')})`);
	});

	it('evaluate like CSS cubic-bezier', () => {
		expect(cubicBezierAt([0, 0, 1, 1], 0.37)).toBeCloseTo(0.37, 6);
		expect(cubicBezierAt(bezierPoints('inOut'), 0.5)).toBeCloseTo(0.5, 5);
		expect(cubicBezierAt(bezierPoints('enter'), 0)).toBe(0);
		expect(cubicBezierAt(bezierPoints('enter'), 1)).toBe(1);
		// ease-out curves are ahead of linear, ease-in behind
		expect(cubicBezierAt(bezierPoints('out'), 0.3)).toBeGreaterThan(0.6);
		expect(cubicBezierAt(bezierPoints('in'), 0.3)).toBeLessThan(0.1);
		// overshooting curves leave [0, 1]
		expect(cubicBezierAt(bezierPoints('pop'), 0.6)).toBeGreaterThan(1);
	});

	it('are monotonic in x for the named curves', () => {
		for (const name of ['standard', 'enter', 'exit', 'inOut', 'sine', 'snap'] as const) {
			const p = bezierPoints(name);
			let last = -1;
			for (let i = 0; i <= 50; i++) {
				const y = cubicBezierAt(p, i / 50);
				expect(y).toBeGreaterThanOrEqual(last - 1e-9);
				last = y;
			}
		}
	});
});

describe('springs', () => {
	it('settle on 1 and keep the tokens', () => {
		for (const cfg of Object.values(SPRING)) {
			const s = spring(cfg);
			expect(s.values[0]).toBeCloseTo(0, 9);
			expect(s.values[s.values.length - 1]).toBe(1);
			expect(s.duration).toBeGreaterThan(0.1);
			expect(s.duration).toBeLessThan(2);
		}
	});

	it('overshoot only when underdamped enough', () => {
		const max = (v: number[]) => Math.max(...v);
		expect(max(spring(SPRING.bouncy).values)).toBeGreaterThan(1.05);
		expect(max(spring(SPRING.smooth).values)).toBeLessThan(1.01);
	});
});
