import * as api from './api';
import type { Preferences, SubtitleSettings } from './api';

export type { SubtitleSettings } from './api';

export const SUBTITLE_FONTS: Record<SubtitleSettings['fontFamily'], string> = {
	sans: 'var(--font-sans)',
	serif: 'Georgia, "Times New Roman", serif',
	mono: 'ui-monospace, "SF Mono", Menlo, monospace',
	rounded: '"Trebuchet MS", "Segoe UI", system-ui, sans-serif'
};

export const DEFAULT_SUBTITLES: SubtitleSettings = {
	fontSizePct: 100,
	color: '#ffffff',
	fontFamily: 'sans',
	backgroundOpacity: 55
};

class UserPreferences {
	subtitles = $state<SubtitleSettings>({ ...DEFAULT_SUBTITLES });
	/** saved display-language code, reconciled into Paraglide on boot */
	language = $state<string | null>(null);
	/** appear on public profiles and leaderboards; on unless turned off */
	publicProfile = $state(true);

	async init() {
		try {
			const prefs = await api.getPreferences();
			this.apply(prefs);
		} catch {
			// unauthenticated or offline: keep defaults
		}
	}

	private apply(prefs: Preferences) {
		this.subtitles = { ...DEFAULT_SUBTITLES, ...(prefs.subtitles ?? {}) };
		this.language = typeof prefs.language === 'string' ? prefs.language : null;
		this.publicProfile = prefs.publicProfile !== false;
	}

	/** persist the current subtitle settings to the account */
	async saveSubtitles(next: SubtitleSettings) {
		this.subtitles = next;
		await api.putPreferences({ subtitles: next });
	}

	/** a lone switch that persists on flip, like the subtitle popover controls */
	async savePublicProfile(next: boolean) {
		this.publicProfile = next;
		await api.putPreferences({ publicProfile: next });
	}
}

export const preferences = new UserPreferences();
