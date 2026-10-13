import { describe, expect, it } from 'vitest';

describe('the package', () => {
	// the web's unit tests import it in Node, so no module may touch the DOM as it loads
	it('loads where there is no DOM', async () => {
		const pkg = await import('../../src/index');
		expect(pkg.TEMPLATE_NAMES).toContain('ident');
		expect(pkg.IDENT_DESIGNS).toEqual(['journey', 'cinematic', 'morph', 'assemble']);
	});
});
