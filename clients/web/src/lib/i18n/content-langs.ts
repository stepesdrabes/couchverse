// Content (metadata/audio) languages offered when configuring a title. Labels
// are endonyms - the same in any UI locale - so they need no translation.
export const CONTENT_LANGS = [
	{ code: 'en', label: 'English' },
	{ code: 'cs', label: 'Čeština' },
	{ code: 'sk', label: 'Slovenčina' },
	{ code: 'de', label: 'Deutsch' },
	{ code: 'es', label: 'Español' },
	{ code: 'fr', label: 'Français' },
	{ code: 'it', label: 'Italiano' },
	{ code: 'pl', label: 'Polski' },
	{ code: 'ko', label: '한국어' },
	{ code: 'ja', label: '日本語' }
];

// label for a code: the curated endonym, else the browser's language name for
// any ISO code (so custom languages read nicely), else the uppercased code.
export const langLabel = (code: string): string => {
	const known = CONTENT_LANGS.find((l) => l.code === code);
	if (known) return known.label;
	try {
		return new Intl.DisplayNames([code], { type: 'language' }).of(code) ?? code.toUpperCase();
	} catch {
		return code.toUpperCase();
	}
};

// All ISO 639-1 codes, offered by the "Add language" search. Only the codes are
// bundled (offline, no key); display names come from langLabel, so there is no
// hand-maintained name list to drift.
// prettier-ignore
export const ALL_LANG_CODES: string[] = [
	'aa','ab','ae','af','ak','am','an','ar','as','av','ay','az','ba','be','bg','bh','bi','bm',
	'bn','bo','br','bs','ca','ce','ch','co','cr','cs','cu','cv','cy','da','de','dv','dz','ee',
	'el','en','eo','es','et','eu','fa','ff','fi','fj','fo','fr','fy','ga','gd','gl','gn','gu',
	'gv','ha','he','hi','ho','hr','ht','hu','hy','hz','ia','id','ie','ig','ii','ik','io','is',
	'it','iu','ja','jv','ka','kg','ki','kj','kk','kl','km','kn','ko','kr','ks','ku','kv','kw',
	'ky','la','lb','lg','li','ln','lo','lt','lu','lv','mg','mh','mi','mk','ml','mn','mr','ms',
	'mt','my','na','nb','nd','ne','ng','nl','nn','no','nr','nv','ny','oc','oj','om','or','os',
	'pa','pi','pl','ps','pt','qu','rm','rn','ro','ru','rw','sa','sc','sd','se','sg','si','sk',
	'sl','sm','sn','so','sq','sr','ss','st','su','sv','sw','ta','te','tg','th','ti','tk','tl',
	'tn','to','tr','ts','tt','tw','ty','ug','uk','ur','uz','ve','vi','vo','wa','wo','xh','yi',
	'yo','za','zh','zu'
];

// English name for a code, for search (so a Czech UI still matches "german").
const englishName = (code: string): string => {
	try {
		return new Intl.DisplayNames(['en'], { type: 'language' }).of(code) ?? '';
	} catch {
		return '';
	}
};

// lazily-built search index: code + endonym label + English name, lower-cased.
let index: { code: string; haystack: string }[] | null = null;

/** ISO codes whose code, label or English name contains the query (all when blank). */
export const searchLangs = (query: string): string[] => {
	index ??= ALL_LANG_CODES.map((code) => ({
		code,
		haystack: `${code} ${langLabel(code)} ${englishName(code)}`.toLowerCase()
	}));
	const q = query.trim().toLowerCase();
	if (!q) return ALL_LANG_CODES;
	return index.filter((e) => e.haystack.includes(q)).map((e) => e.code);
};
