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
