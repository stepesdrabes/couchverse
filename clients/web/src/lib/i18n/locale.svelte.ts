import { getLocale, setLocale, locales, isLocale } from '$lib/paraglide/runtime';

export type DisplayLang = (typeof locales)[number];

/** supported display languages (compile-time set from project.inlang) */
export const displayLangs = locales as readonly DisplayLang[];

const LABELS: Record<string, string> = { en: 'English', cs: 'Čeština' };

/** human label for a language code */
export const displayLangLabel = (lang: string): string => LABELS[lang] ?? lang.toUpperCase();

/** the active display language - read by the API client on every request */
export const currentLang = (): DisplayLang => getLocale();

/**
 * Switch language. Paraglide persists the choice to localStorage and reloads the
 * page, so every message and every ?lang= data fetch move together.
 */
export const setDisplayLang = (lang: DisplayLang): void => {
	if (lang !== getLocale()) setLocale(lang);
};

/** reconcile a saved account preference on boot (reloads once if it differs) */
export const applySavedLang = (lang: unknown): void => {
	if (typeof lang === 'string' && isLocale(lang) && lang !== getLocale()) setLocale(lang);
};
