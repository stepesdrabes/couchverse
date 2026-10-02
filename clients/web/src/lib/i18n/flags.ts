// Flag for a content/display language: maps an ISO 639-1 language code to the
// ISO 3166-1 alpha-2 country code that flag-icons draws. A language often has no
// single country (en, es, ar, ...), so these are conventional picks; codes with
// no sensible flag return null and the UI shows the label alone.
const LANG_COUNTRY: Record<string, string> = {
	en: 'gb', // English -> United Kingdom (conventional)
	cs: 'cz',
	sk: 'sk',
	de: 'de',
	es: 'es',
	fr: 'fr',
	it: 'it',
	pl: 'pl',
	pt: 'pt',
	nl: 'nl',
	ru: 'ru',
	uk: 'ua',
	ja: 'jp',
	ko: 'kr',
	zh: 'cn',
	hi: 'in',
	ar: 'sa',
	he: 'il',
	tr: 'tr',
	el: 'gr',
	sv: 'se',
	da: 'dk',
	nb: 'no',
	nn: 'no',
	no: 'no',
	fi: 'fi',
	hu: 'hu',
	ro: 'ro',
	bg: 'bg',
	hr: 'hr',
	sr: 'rs',
	sl: 'si',
	et: 'ee',
	lv: 'lv',
	lt: 'lt',
	is: 'is',
	ga: 'ie',
	id: 'id',
	ms: 'my',
	th: 'th',
	vi: 'vn',
	fa: 'ir',
	fil: 'ph',
	tl: 'ph'
};

/** ISO 3166-1 alpha-2 country for a language code, or null when there is no sensible flag. */
export const langToCountry = (code: string): string | null =>
	LANG_COUNTRY[code.toLowerCase()] ?? null;
