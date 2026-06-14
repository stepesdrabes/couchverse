import type { Job } from './api';
import * as m from '$lib/paraglide/messages';

const actionByType: Record<string, () => string> = {
	scan_library: m.jobs_action_scan_library,
	probe: m.jobs_action_probe,
	transcode_hls: m.jobs_action_transcode,
	extract_subtitles: m.jobs_action_extract_subtitles,
	fetch_metadata: m.jobs_action_fetch_metadata,
	import_episodes: m.jobs_action_import_episodes,
	cleanup: m.jobs_action_cleanup
};

const pad = (n: number) => String(n).padStart(2, '0');

/** "Transcode 1080p", "Analyze", "Scan library" */
export function jobAction(job: Job): string {
	const base = actionByType[job.type]?.() ?? job.type.replaceAll('_', ' ');
	const variant = job.subject?.variant;
	return job.type === 'transcode_hls' && variant ? `${base} ${variant}` : base;
}

/** "The Simpsons S01E01", "Movie Name", "Track Name" - null when unknown */
export function jobSubjectLabel(job: Job): string | null {
	const s = job.subject;
	if (s?.trackName) return s.trackName;
	if (!s?.titleName) return null;
	if (s.seasonNumber != null && s.episodeNumber != null) {
		return `${s.titleName} S${pad(s.seasonNumber)}E${pad(s.episodeNumber)}`;
	}
	return s.titleName;
}
