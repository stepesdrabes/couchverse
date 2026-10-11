// Couch TV admin data. The broadcast preview reads the library through the existing admin
// operations; the channel's own operations arrive with the schedule.
export { adminGetTitle, adminListLibrary } from '$lib/generated/api';
export type { AdminTitle, Artwork, LibraryRow } from '$lib/generated/api';
