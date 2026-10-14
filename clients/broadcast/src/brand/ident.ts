// The channel ident: the full cut (10.52 s) is one of four designs (idents/, D35), the short cut
// (5.2 s) the popcorn ending they all close on, alone (short-ident.ts, D34).

import type { IdentSpec } from '../spec';
import type { Template } from '../templates/shared';
import { buildFullIdent } from './idents';
import { buildShortIdent } from './short-ident';

export const ident: Template<IdentSpec> = {
	version: 8,
	build(ctx, spec) {
		return spec.cut === 'short' ? buildShortIdent(ctx) : buildFullIdent(ctx, spec);
	}
};
