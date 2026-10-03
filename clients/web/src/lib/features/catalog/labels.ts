// The core hands over codes, never UI words; these are their labels in the display language.

import { HomeRowKind, Quality, TitleKind } from '$lib/generated/core';
import * as m from '$lib/paraglide/messages';

export const kindLabel = (kind: TitleKind): string =>
	kind === TitleKind.Series ? m.catalog_kind_series() : m.catalog_kind_movie();

export function qualityLabel(quality: Quality): string {
	switch (quality) {
		case Quality.Uhd:
			return m.catalog_quality_uhd();
		case Quality.Hd1080:
			return m.catalog_quality_hd1080();
		case Quality.Hd720:
			return m.catalog_quality_hd720();
		case Quality.Sd:
			return m.catalog_quality_sd();
	}
}

// The built-in rows keep English default labels in the database; those are translated by kind,
// while a label the admin customized in the home-row editor is shown as it is.
const ROW_DEFAULTS: Partial<Record<HomeRowKind, string>> = {
	[HomeRowKind.ContinueWatching]: 'Continue Watching',
	[HomeRowKind.RecentlyAdded]: 'Up on the Marquee'
};

export function rowLabel(row: { kind: HomeRowKind; label: string }): string {
	if (row.label !== ROW_DEFAULTS[row.kind]) return row.label;
	switch (row.kind) {
		case HomeRowKind.ContinueWatching:
			return m.home_row_continue_watching();
		case HomeRowKind.RecentlyAdded:
			return m.home_row_recently_added();
		default:
			return row.label;
	}
}
