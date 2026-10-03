// Catalog entities the admin panel shares. The viewer's catalog reads the core's view models
// (`Card`, `HomeView`, `TitleView`... in `$lib/generated/core`).

export type ContentStatus = 'draft' | 'processing' | 'published' | 'hidden';

export interface Genre {
	id: number;
	name: string;
	label: string;
}
