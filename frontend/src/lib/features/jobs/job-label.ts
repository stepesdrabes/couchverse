import type { Job } from './api';

const actionByType: Record<string, string> = {
	scan_library: 'Scan library',
	probe: 'Analyze',
	transcode_hls: 'Transcode',
	extract_subtitles: 'Extract subtitles',
	fetch_metadata: 'Fetch metadata',
	import_episodes: 'Import episodes',
	cleanup: 'Cleanup'
};

const pad = (n: number) => String(n).padStart(2, '0');

/** "Transcode 1080p", "Analyze", "Scan library" */
export function jobAction(job: Job): string {
	const base = actionByType[job.type] ?? job.type.replaceAll('_', ' ');
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
