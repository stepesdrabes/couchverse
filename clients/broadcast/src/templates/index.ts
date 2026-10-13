import { ident } from '../brand/ident';
import { sting } from '../brand/sting';
import type { ClipSpec, TemplateName } from '../spec';
import { lineup } from './lineup';
import { news } from './news';
import { evergreenEnd, promo } from './promo';
import type { Template } from './shared';
import { shortPromo } from './short';
import { squeeze } from './squeeze';
import { standby } from './standby';
import { stripPromo } from './strip';
import { upNext } from './upnext';
import { weather } from './weather';

type Registry = { [K in TemplateName]: Template<Extract<ClipSpec, { template: K }>> };

/** Every template by its spec's `template` name. */
export const TEMPLATES: Registry = {
	ident,
	sting,
	upNext,
	promo,
	evergreenEnd,
	stripPromo,
	shortPromo,
	lineup,
	squeeze,
	standby,
	weather,
	news
};

export const TEMPLATE_NAMES = Object.keys(TEMPLATES) as TemplateName[];

export function templateOf<S extends ClipSpec>(spec: S): Template<S> {
	const t = TEMPLATES[spec.template] as unknown as Template<S> | undefined;
	if (!t) throw new Error(`unknown template: ${String((spec as { template?: unknown }).template)}`);
	return t;
}
