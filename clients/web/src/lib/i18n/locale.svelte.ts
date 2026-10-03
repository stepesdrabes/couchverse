import { core } from '$lib/core';
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
 * Switch language. The core saves it to the account first; then Paraglide persists the
 * choice to localStorage and reloads the page, so every message and every ?lang= data fetch
 * move together.
 */
export async function setDisplayLang(lang: DisplayLang): Promise<void> {
	if (lang === getLocale()) return;
	await core.send({ type: 'displayLanguageChanged', content: { code: lang } });
	setLocale(lang);
}

/**
 * Adopt the signed-in account's language from the core's session. Returns true when it
 * differs: the page is reloading into it.
 */
export function followAccountLang(): boolean {
	const lang = core.session.language;
	if (!isLocale(lang) || lang === getLocale()) return false;
	setLocale(lang);
	return true;
}
